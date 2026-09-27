// Copyright (c) 2026 Naawaakrit
// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (a *app_) onStart() {
	if a.running {
		return
	}
	if len(a.jobs) == 0 {
		dialog.ShowInformation("ไม่มีไฟล์", "กรุณาเลือกไฟล์หรือโฟลเดอร์ต้นฉบับก่อน", a.win)
		return
	}
	if a.destDir == "" {
		dialog.ShowInformation("ไม่มีปลายทาง", "กรุณาเลือกโฟลเดอร์ปลายทางก่อน", a.win)
		return
	}
	if err := validateDestination(a.destDir, a.sources, a.jobs); err != nil {
		dialog.ShowInformation("ปลายทางไม่ปลอดภัย", err.Error(), a.win)
		return
	}

	for _, j := range a.jobs {
		j.Status = statusWaiting
		j.Err = nil
	}
	a.ctrl.reset()
	a.running = true
	a.btnStart.Disable()
	a.btnRemoveJob.Disable()
	a.btnPause.Enable()
	a.btnPause.SetText("หยุดชั่วคราว")
	a.btnCancel.Enable()

	go a.runCopy()
}

func (a *app_) onPauseResume() {
	if !a.running {
		return
	}
	paused := a.ctrl.togglePause()
	if paused {
		a.btnPause.SetText("ทำต่อ")
	} else {
		a.btnPause.SetText("หยุดชั่วคราว")
	}
}

func (a *app_) onCancel() {
	if !a.running {
		return
	}
	a.ctrl.cancel()
}

func (a *app_) runCopy() {
	var totalBytes, doneBytes int64
	for _, j := range a.jobs {
		totalBytes += j.Size
	}

	a.errorLog = nil
	sessionStart := time.Now()

	fyne.Do(func() {
		a.etaLabel.SetText("")
		a.speedLabel.SetText("")
	})

	doneCount := 0
	for idx, j := range a.jobs {
		if a.ctrl.isCancelled() {
			j.Status = statusSkipped
			continue
		}

		j.Status = statusCopying
		j.Retries = 0
		a.safeRefreshFileList()
		relPath := j.RelPath
		fyne.Do(func() {
			a.currentLabel.SetText(fmt.Sprintf("กำลังคัดลอก: %s", relPath))
			a.fileProgress.SetValue(0)
		})

		destPath := filepath.Join(a.destDir, j.RelPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			j.Status = statusError
			j.Err = err
			a.errorLog = append(a.errorLog, fmt.Sprintf("[Error] %s\n  → %s", relPath, err.Error()))
			continue
		}

		finalDstPath, action := a.resolveDestination(j.SrcPath, destPath, j.RelPath)
		if action == "skip" {
			j.Status = statusSkipped
			a.safeRefreshFileList()
			continue
		}

		lastChunkTime := time.Now()
		lastCopied := int64(0)
		var currentSpeed float64

		progressFn := func(copied int64) {
			if j.Size > 0 {
				val := float64(copied) / float64(j.Size)
				fyne.Do(func() {
					a.fileProgress.SetValue(val)
				})
			}

			now := time.Now()
			elapsed := now.Sub(lastChunkTime).Seconds()
			if elapsed >= 0.4 {
				bytesDiff := copied - lastCopied
				instSpeed := float64(bytesDiff) / elapsed
				if currentSpeed == 0 {
					currentSpeed = instSpeed
				} else {
					alpha := 0.3
					currentSpeed = alpha*instSpeed + (1-alpha)*currentSpeed
				}

				sessionElapsed := now.Sub(sessionStart).Seconds()
				totalDone := doneBytes + copied
				var etaStr string
				if sessionElapsed > 1 && totalDone > 0 {
					sessionSpeed := float64(totalDone) / sessionElapsed
					remBytes := totalBytes - totalDone
					if remBytes > 0 && sessionSpeed > 0 {
						remSec := int(float64(remBytes) / sessionSpeed)
						etaStr = formatDuration(remSec)
					} else if remBytes <= 0 {
						etaStr = "เสร็จแล้ว"
					}
				}

				elapsedStr := formatDuration(int(sessionElapsed))
				speedStr := fmt.Sprintf("ความเร็ว: %s/วินาที", humanSize(int64(currentSpeed)))
				etaDisplay := ""
				if etaStr != "" {
					etaDisplay = fmt.Sprintf("⏱ ผ่านมา %s  |  เหลือ ~%s", elapsedStr, etaStr)
				} else {
					etaDisplay = fmt.Sprintf("⏱ ผ่านมา %s", elapsedStr)
				}
				fyne.Do(func() {
					a.speedLabel.SetText(speedStr)
					a.etaLabel.SetText(etaDisplay)
				})
				lastChunkTime = now
				lastCopied = copied
			}
			a.updateOverall(doneCount, len(a.jobs), doneBytes+copied, totalBytes)
		}

		var copiedInFile int64
		var copyErr error
		for attempt := 0; ; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Second)
				if a.ctrl.isCancelled() {
					copyErr = errCancelled
					break
				}
				j.Status = statusRetrying
				attemptNum := attempt
				fyne.Do(func() {
					a.currentLabel.SetText(fmt.Sprintf("Retry (%d/%d): %s", attemptNum, a.maxRetry, relPath))
					a.fileProgress.SetValue(0)
				})
				a.safeRefreshFileList()
			}

			copiedInFile, copyErr = a.copyOneFile(j.SrcPath, finalDstPath, j.Size, progressFn)
			if copyErr == nil || copyErr == errCancelled {
				break
			}

			j.Retries = attempt + 1
			if attempt >= a.maxRetry {
				break
			}
		}

		if copyErr != nil {
			if copyErr == errCancelled {
				j.Status = statusSkipped
			} else {
				j.Status = statusError
				j.Err = copyErr
				errMsg := fmt.Sprintf("[Error] %s (retry %d/%d)\n  → %s",
					relPath, j.Retries, a.maxRetry, copyErr.Error())
				a.errorLog = append(a.errorLog, errMsg)
			}
		} else if a.verify != verifyNone {
			j.Status = statusVerifying
			a.safeRefreshFileList()
			fyne.Do(func() {
				a.currentLabel.SetText(fmt.Sprintf("กำลังตรวจสอบ Hash: %s", relPath))
			})
			vErr := a.verifyHash(j.SrcPath, finalDstPath)
			if vErr != nil {
				j.Status = statusVerifyFailed
				j.Err = vErr
				a.errorLog = append(a.errorLog, fmt.Sprintf("[VerifyFailed] %s\n  → %s", relPath, vErr.Error()))
			} else {
				j.Status = statusDone
				doneBytes += copiedInFile
				doneCount++
			}
		} else {
			j.Status = statusDone
			doneBytes += copiedInFile
			doneCount++
		}

		a.safeRefreshFileList()
		a.updateOverall(doneCount, len(a.jobs), doneBytes, totalBytes)

		if a.ctrl.isCancelled() {
			for _, rest := range a.jobs[idx+1:] {
				rest.Status = statusSkipped
			}
			a.safeRefreshFileList()
			break
		}
	}

	a.running = false
	errSnapshot := a.errorLog

	fyne.Do(func() {
		a.currentLabel.SetText("เสร็จสิ้น (หรือถูกยกเลิก)")
		a.btnStart.Enable()
		a.btnPause.Disable()
		a.btnCancel.Disable()
		if a.selectedJob >= 0 && a.selectedJob < len(a.jobs) {
			a.btnRemoveJob.Enable()
		}

		if len(errSnapshot) > 0 {
			summary := fmt.Sprintf("พบ %d ไฟล์ที่เกิดข้อผิดพลาด:\n\n", len(errSnapshot))
			for i, e := range errSnapshot {
				summary += fmt.Sprintf("%d. %s\n\n", i+1, e)
			}
			logLbl := widget.NewLabel(summary)
			logLbl.Wrapping = fyne.TextWrapWord
			scroll := container.NewVScroll(logLbl)
			scroll.SetMinSize(fyne.NewSize(560, 300))
			d := dialog.NewCustom("Error Log - สรุปไฟล์ที่ผิดพลาด", "ปิด", scroll, a.win)
			d.Resize(fyne.NewSize(600, 400))
			d.Show()
		}
	})
}

var errCancelled = fmt.Errorf("ยกเลิกโดยผู้ใช้")

const copyBufSize = 1024 * 1024

func (a *app_) copyOneFile(src, dst string, size int64, onProgress func(copied int64)) (int64, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if dstInfo, err := os.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
		return 0, fmt.Errorf("ไฟล์ปลายทางเป็นไฟล์เดียวกับต้นทาง: %s", src)
	}

	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	srcInfo, err = in.Stat()
	if err == nil && a.preserveMetadata {
		_ = out.Chmod(srcInfo.Mode())
	}

	buf := make([]byte, copyBufSize)
	var copied int64
	for {
		if a.ctrl.waitIfPaused() || a.ctrl.isCancelled() {
			out.Close()
			os.Remove(dst)
			return copied, errCancelled
		}

		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(dst)
				return copied, werr
			}
			copied += int64(n)
			if onProgress != nil {
				onProgress(copied)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(dst)
			return copied, rerr
		}
	}

	if srcInfo != nil && a.preserveMetadata {
		_ = out.Close()
		_ = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	}

	return copied, nil
}

func validateDestination(destDir string, sources []string, jobs []*copyJob) error {
	resolvedDest, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		return fmt.Errorf("ไม่สามารถตรวจสอบโฟลเดอร์ปลายทางได้: %w", err)
	}
	resolvedDest, err = filepath.Abs(resolvedDest)
	if err != nil {
		return err
	}

	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil || !info.IsDir() {
			continue
		}
		resolvedSource, err := filepath.EvalSymlinks(src)
		if err != nil {
			continue
		}
		resolvedSource, err = filepath.Abs(resolvedSource)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(resolvedSource, resolvedDest)
		if err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
			return fmt.Errorf("เลือกโฟลเดอร์ปลายทางที่ตรงกับหรืออยู่ภายในโฟลเดอร์ต้นทางไม่ได้: %s", src)
		}
	}

	for _, job := range jobs {
		srcInfo, err := os.Stat(job.SrcPath)
		if err != nil {
			continue
		}
		dstInfo, err := os.Stat(filepath.Join(destDir, job.RelPath))
		if err == nil && os.SameFile(srcInfo, dstInfo) {
			return fmt.Errorf("ไฟล์ปลายทางตรงกับไฟล์ต้นฉบับ: %s", job.SrcPath)
		}
	}
	return nil
}

func (a *app_) resolveDestination(srcPath, destPath, relPath string) (string, string) {
	dstInfo, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return destPath, "copy"
	}

	switch a.policy {
	case overwriteAlways:
		return destPath, "copy"
	case overwriteSkip:
		return destPath, "skip"
	case overwriteRename:
		return generateUniqueName(destPath), "copy"
	case overwriteAsk:
		srcInfo, sErr := os.Stat(srcPath)

		srcSizeStr, srcTimeStr := "ไม่ทราบ", "ไม่ทราบ"
		if sErr == nil {
			srcSizeStr = humanSize(srcInfo.Size())
			srcTimeStr = srcInfo.ModTime().Format("2006-01-02 15:04:05")
		}

		dstSizeStr, dstTimeStr := "ไม่ทราบ", "ไม่ทราบ"
		if err == nil {
			dstSizeStr = humanSize(dstInfo.Size())
			dstTimeStr = dstInfo.ModTime().Format("2006-01-02 15:04:05")
		}

		type result struct {
			action       string
			targetPath   string
			applyAll     bool
			chosenPolicy overwritePolicy
		}
		ch := make(chan result)

		fyne.Do(func() {
			msgLabel := widget.NewLabel(fmt.Sprintf("พบไฟล์ปลายทางที่มีชื่อซ้ำกัน:\n%s", relPath))
			msgLabel.TextStyle = fyne.TextStyle{Bold: true}

			srcBox := container.NewVBox(
				widget.NewLabelWithStyle("ต้นทาง (Source)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(fmt.Sprintf("ขนาด: %s", srcSizeStr)),
				widget.NewLabel(fmt.Sprintf("แก้ไขล่าสุด: %s", srcTimeStr)),
			)

			dstBox := container.NewVBox(
				widget.NewLabelWithStyle("ปลายทางเดิม (Destination)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(fmt.Sprintf("ขนาด: %s", dstSizeStr)),
				widget.NewLabel(fmt.Sprintf("แก้ไขล่าสุด: %s", dstTimeStr)),
			)

			comparisonContainer := container.NewGridWithColumns(2, srcBox, dstBox)
			applyAllCheck := widget.NewCheck("นำตัวเลือกนี้ไปใช้กับไฟล์ซ้ำที่เหลือทั้งหมดในรอบนี้ (Apply to all)", nil)

			var d dialog.Dialog

			btnOverwrite := widget.NewButton("เขียนทับ (Overwrite)", func() {
				d.Hide()
				ch <- result{action: "copy", targetPath: destPath, applyAll: applyAllCheck.Checked, chosenPolicy: overwriteAlways}
			})
			btnOverwrite.Importance = widget.HighImportance

			btnRename := widget.NewButton("เปลี่ยนชื่ออัตโนมัติ (Rename)", func() {
				d.Hide()
				ch <- result{action: "copy", targetPath: generateUniqueName(destPath), applyAll: applyAllCheck.Checked, chosenPolicy: overwriteRename}
			})

			btnSkip := widget.NewButton("ข้ามไฟล์นี้ (Skip)", func() {
				d.Hide()
				ch <- result{action: "skip", targetPath: destPath, applyAll: applyAllCheck.Checked, chosenPolicy: overwriteSkip}
			})

			btnBox := container.NewHBox(btnOverwrite, btnRename, btnSkip)
			content := container.NewVBox(
				msgLabel,
				widget.NewSeparator(),
				comparisonContainer,
				widget.NewSeparator(),
				applyAllCheck,
				btnBox,
			)

			d = dialog.NewCustomWithoutButtons("พบไฟล์ซ้ำในปลายทาง (File Conflict)", content, a.win)
			d.Show()
		})

		res := <-ch
		if res.applyAll {
			a.policy = res.chosenPolicy
		}
		return res.targetPath, res.action
	}

	return destPath, "copy"
}

func generateUniqueName(destPath string) string {
	ext := filepath.Ext(destPath)
	base := strings.TrimSuffix(destPath, ext)
	counter := 1
	for {
		newName := fmt.Sprintf("%s (%d)%s", base, counter, ext)
		if _, err := os.Stat(newName); os.IsNotExist(err) {
			return newName
		}
		counter++
	}
}

func (a *app_) safeRefreshFileList() {
	fyne.Do(func() {
		if a.fileList != nil {
			a.fileList.Refresh()
		}
	})
}
