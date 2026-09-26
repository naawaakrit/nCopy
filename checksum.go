package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

func (a *app_) verifyHash(src, dst string) error {
	srcHash, err := a.computeFileHash(src)
	if err != nil {
		return fmt.Errorf("คำนวณ Hash ต้นทางล้มเหลว: %w", err)
	}

	dstHash, err := a.computeFileHash(dst)
	if err != nil {
		return fmt.Errorf("คำนวณ Hash ปลายทางล้มเหลว: %w", err)
	}

	if srcHash != dstHash {
		return fmt.Errorf("Checksum ไม่ตรงกัน! (ต้นทาง: %s..., ปลายทาง: %s...)",
			truncateHash(srcHash), truncateHash(dstHash))
	}

	return nil
}

func (a *app_) computeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	if a.verify == verifySHA256 {
		h = sha256.New()
	} else {
		h = md5.New()
	}

	buf := make([]byte, copyBufSize)
	for {
		if a.ctrl.waitIfPaused() || a.ctrl.isCancelled() {
			return "", errCancelled
		}
		n, rerr := f.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return "", werr
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func truncateHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
