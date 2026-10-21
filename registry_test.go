package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileStaysWithinRoot(t *testing.T) {
	root := t.TempDir()

	outside := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	got, err := readFile(context.Background(), `{"path":"ok.txt"}`, root)
	if err != nil || got != "hello" {
		t.Fatalf("读取目录内文件: got=%q, err=%v", got, err)
	}

	for _, args := range []string{
		`{"path":"../secret.txt"}`,
		`{"path":"link.txt"}`,
	} {
		if got, err := readFile(context.Background(), args, root); err == nil {
			t.Errorf("越界读取应失败: args=%s, got=%q", args, got)
		}
	}
}

func TestReadFileRejectsBinary(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "binary.dat"), []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}

	if got, err := readFile(context.Background(), `{"path":"binary.dat"}`, root); err == nil {
		t.Fatalf("应拒绝二进制文件,实际返回 %q", got)
	}
}

func TestReadFileSizeLimit(t *testing.T) {
	root := t.TempDir()
	const limit = 64 << 10

	for _, tc := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"limit.txt", limit, false},
		{"too-large.txt", limit + 1, true},
	} {
		data := []byte(strings.Repeat("a", tc.size))
		if err := os.WriteFile(filepath.Join(root, tc.name), data, 0o600); err != nil {
			t.Fatal(err)
		}

		args := `{"path":"` + tc.name + `"}`
		got, err := readFile(context.Background(), args, root)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: 返回错误 %v, 期望错误=%v", tc.name, err, tc.wantErr)
		}
		if !tc.wantErr && len(got) != limit {
			t.Errorf("%s: 读取了 %d 字节, 期望 %d", tc.name, len(got), limit)
		}
	}
}
