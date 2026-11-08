package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Extraction is capped so a zip/gzip bomb cannot exhaust the disk.
// shortcut: 7z/rar extraction is delegated to 7z/unar and stays uncapped.
// Upgrade when untrusted 7z/rar archives become a real input.
const (
	defaultMaxExtractBytes   = int64(2) << 30 // 2 GiB
	defaultMaxExtractEntries = 200000
)

type extractBudget struct {
	bytes      int64
	entries    int
	maxBytes   int64
	maxEntries int
}

func newExtractBudget() *extractBudget {
	b := &extractBudget{maxBytes: defaultMaxExtractBytes, maxEntries: defaultMaxExtractEntries}
	if n, err := strconv.ParseInt(os.Getenv("FILE_CLI_MAX_EXTRACT_MB"), 10, 64); err == nil && n > 0 {
		b.maxBytes = n << 20
	}
	if n, err := strconv.Atoi(os.Getenv("FILE_CLI_MAX_EXTRACT_ENTRIES")); err == nil && n > 0 {
		b.maxEntries = n
	}
	return b
}

func (b *extractBudget) entry() error {
	b.entries++
	if b.entries > b.maxEntries {
		return fmt.Errorf("archive has more than %d entries", b.maxEntries)
	}
	return nil
}

func (b *extractBudget) add(n int64) error {
	b.bytes += n
	if b.bytes > b.maxBytes {
		return fmt.Errorf("archive expands past %d MiB", b.maxBytes>>20)
	}
	return nil
}

// convertArchive extracts the source into a temp directory and repacks it.
// shortcut: archives are staged on disk, not streamed entry to entry.
// Upgrade when someone converts multi-gigabyte archives.
func convertArchive(ctx context.Context, src string, k Kind, t Target, out string, emit func(Progress)) error {
	tmp, err := os.MkdirTemp("", "file-tui-arc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	emit(Progress{Percent: -1, Label: "reading archive"})
	if err := extractArchive(ctx, src, k, tmp); err != nil {
		return err
	}
	emit(Progress{Percent: -1, Label: "writing ." + t.Ext})
	return packArchive(ctx, tmp, t.Ext, out, emit)
}

func extractArchive(ctx context.Context, src string, k Kind, dst string) error {
	budget := newExtractBudget()
	switch k.ID {
	case "zip":
		zr, err := zip.OpenReader(src)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, zf := range zr.File {
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := budget.entry(); err != nil {
				return err
			}
			target, ok := safeJoin(dst, zf.Name)
			if !ok {
				continue
			}
			info := zf.FileInfo()
			if info.IsDir() {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			if err := writeStream(target, rc, info.Mode().Perm(), budget); err != nil {
				rc.Close()
				return fmt.Errorf("extract %s: %w", target, err)
			}
		}
		return nil

	case "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst":
		r, wait, err := openTarStream(ctx, src, k.ID)
		if err != nil {
			return err
		}
		// wait must run even on an early cancel, or the decompressor is never
		// reaped and its stdout pipe leaks.
		waited := false
		defer func() {
			if !waited {
				_ = wait()
			}
		}()
		tr := tar.NewReader(r)
		for {
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := budget.entry(); err != nil {
				return err
			}
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			target, ok := safeJoin(dst, hdr.Name)
			if !ok {
				continue
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			case tar.TypeReg:
				if err := writeStream(target, tr, fs.FileMode(hdr.Mode).Perm(), budget); err != nil {
					return fmt.Errorf("extract %s: %w", target, err)
				}
			}
		}
		_, _ = io.Copy(io.Discard, r)
		waited = true
		return wait()

	case "gz", "bz2", "xz", "zst":
		r, wait, err := openCompressed(ctx, src, k.ID)
		if err != nil {
			return err
		}
		defer wait()
		name := filepath.Base(srcBase(src))
		if name == "" || name == "." {
			name = "decompressed"
		}
		target, ok := safeJoin(dst, name)
		if !ok {
			return fmt.Errorf("unsafe entry name")
		}
		if err := budget.entry(); err != nil {
			return err
		}
		if err := writeStream(target, r, 0o644, budget); err != nil {
			return fmt.Errorf("extract %s: %w", target, err)
		}

	case "7z", "rar":
		if _, err := exec.LookPath("7z"); err == nil {
			if err := runCmd(ctx, "7z", []string{"x", "-y", "-o" + dst, src}, nil); err != nil {
				return err
			}
		} else if _, err := exec.LookPath("unar"); err == nil {
			if err := runCmd(ctx, "unar", []string{"-q", "-o", dst, src}, nil); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("install 7zip to read .%s archives", k.ID)
		}
		// 7z/unar have no built-in extraction budget, so verify the result
		// against the same byte/entry limits applied to zip/tar extraction.
		if err := verifyExtractBudget(dst, budget); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unsupported archive source .%s", k.ID)
}

func openTarStream(ctx context.Context, src, id string) (io.Reader, func() error, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, nil, err
	}
	switch id {
	case "tar":
		return f, f.Close, nil
	case "tar.gz":
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		return gz, func() error { gz.Close(); return f.Close() }, nil
	case "tar.bz2":
		f.Close()
		return execStream(ctx, "bzip2", []string{"-dc", src})
	case "tar.xz":
		f.Close()
		return execStream(ctx, "xz", []string{"-dc", src})
	case "tar.zst":
		f.Close()
		return execStream(ctx, "zstd", []string{"-q", "-dc", src})
	}
	f.Close()
	return nil, nil, fmt.Errorf("unsupported tar compression")
}

func openCompressed(ctx context.Context, src, id string) (io.Reader, func() error, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, nil, err
	}
	if id == "gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		return gz, func() error { gz.Close(); return f.Close() }, nil
	}
	f.Close()
	switch id {
	case "bz2":
		return execStream(ctx, "bzip2", []string{"-dc", src})
	case "xz":
		return execStream(ctx, "xz", []string{"-dc", src})
	case "zst":
		return execStream(ctx, "zstd", []string{"-q", "-dc", src})
	}
	return nil, nil, fmt.Errorf("unsupported compression .%s", id)
}

func execStream(ctx context.Context, bin string, args []string) (io.Reader, func() error, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var errBuf tailBuffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	wait := func() error {
		if err := cmd.Wait(); err != nil {
			msg := lastLine(errBuf.String())
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("%s: %s", bin, msg)
		}
		return nil
	}
	return stdout, wait, nil
}

// packArchive walks the temp dir twice: once to count entries, once to pack.
// This avoids holding every file's metadata in memory at once.
func packArchive(ctx context.Context, root, ext, out string, emit func(Progress)) error {
	total := 0
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		total++
		return nil
	})
	if total == 0 {
		total = 1
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	i := 0
	step := func() {
		i++
		emit(Progress{Percent: i * 100 / total, Label: "writing ." + ext})
	}

	switch ext {
	case "zip":
		zw := zip.NewWriter(f)
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				return err
			}
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := addZipFile(zw, root, p, d); err != nil {
				return err
			}
			step()
			return nil
		})
		if err == nil {
			err = zw.Close()
		}
		return err

	case "tar":
		tw := tar.NewWriter(f)
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				return err
			}
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := packTarFile(tw, root, p, d); err != nil {
				return err
			}
			step()
			return nil
		})
		if err == nil {
			err = tw.Close()
		}
		return err

	case "tar.gz":
		gz := gzip.NewWriter(f)
		tw := tar.NewWriter(gz)
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				return err
			}
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := packTarFile(tw, root, p, d); err != nil {
				return err
			}
			step()
			return nil
		})
		if err == nil {
			err = tw.Close()
		}
		if err == nil {
			err = gz.Close()
		}
		return err

	case "tar.xz", "tar.zst":
		bin, args := "xz", []string{"-z", "-c", "-T0"}
		if ext == "tar.zst" {
			bin, args = "zstd", []string{"-q", "-z", "-c"}
		}
		cmd := exec.CommandContext(ctx, bin, args...)
		var errBuf tailBuffer
		cmd.Stderr = &errBuf
		cmd.Stdout = f
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		tw := tar.NewWriter(stdin)
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				return err
			}
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			if err := packTarFile(tw, root, p, d); err != nil {
				return err
			}
			step()
			return nil
		})
		if perr := tw.Close(); perr != nil && err == nil {
			err = perr
		}
		stdin.Close()
		werr := cmd.Wait()
		if err != nil {
			return err
		}
		if werr != nil {
			return fmt.Errorf("%s: %s", bin, lastLine(errBuf.String()))
		}
		return nil
	}
	return fmt.Errorf("unsupported archive target .%s", ext)
}

func addZipFile(zw *zip.Writer, root, path string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(rel)
	if info.IsDir() {
		hdr.Name += "/"
	} else {
		hdr.Method = zip.Deflate
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

func packTarFile(tw *tar.Writer, root, path string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(rel)
	if info.IsDir() {
		hdr.Name += "/"
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeStream(dst string, r io.Reader, mode fs.FileMode, budget *extractBudget) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	// Read at most one byte past the remaining budget so overflow is detected
	// without decompressing more than the cap.
	n, cerr := io.Copy(f, io.LimitReader(r, budget.maxBytes-budget.bytes+1))
	if cerr == nil {
		cerr = budget.add(n)
	}
	if err := f.Close(); err != nil && cerr == nil {
		cerr = err
	}
	if cerr == nil {
		_ = os.Chmod(dst, mode)
	}
	return cerr
}

// safeJoin blocks path traversal in archive entry names.
func safeJoin(root, name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := path.Clean("/" + name)
	// Reject entries that resolve to the root itself (., .., empty, /).
	if clean == "/" {
		return "", false
	}
	p := filepath.Join(root, filepath.FromSlash(clean))
	if p != root && !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		return "", false
	}
	return p, true
}

// verifyExtractBudget walks dst and checks total size and entry count against
// the budget. Used for 7z/rar extraction which has no built-in limits.
func verifyExtractBudget(dst string, budget *extractBudget) error {
	entries := 0
	var totalBytes int64
	err := filepath.WalkDir(dst, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dst {
			return nil
		}
		entries++
		if entries > budget.maxEntries {
			return fmt.Errorf("archive has more than %d entries", budget.maxEntries)
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			totalBytes += info.Size()
			if totalBytes > budget.maxBytes {
				return fmt.Errorf("archive expands past %d MiB", budget.maxBytes>>20)
			}
		}
		return nil
	})
	return err
}
