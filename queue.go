// Copyright (c) 2026 Naawaakrit
// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
)

func (a *app_) rebuildQueue() {
	var jobs []*copyJob

	for _, src := range a.sources {
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		if info.IsDir() {
			base := filepath.Base(src)
			filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(src, p)
				rel = filepath.Join(base, rel)
				jobs = append(jobs, &copyJob{SrcPath: p, RelPath: rel, Size: fi.Size(), Status: statusWaiting})
				return nil
			})
		} else {
			jobs = append(jobs, &copyJob{SrcPath: src, RelPath: filepath.Base(src), Size: info.Size(), Status: statusWaiting})
		}
	}

	a.applySort(jobs)

	a.jobs = jobs
	a.selectedJob = -1
	a.fileList.UnselectAll()
	a.btnRemoveJob.Disable()
	a.fileList.Refresh()
	var total int64
	for _, j := range jobs {
		total += j.Size
	}
	a.updateOverall(0, len(jobs), 0, total)
}

func (a *app_) removeSelectedJob() {
	if a.running || a.selectedJob < 0 || a.selectedJob >= len(a.jobs) {
		return
	}

	idx := a.selectedJob
	a.jobs = append(a.jobs[:idx], a.jobs[idx+1:]...)
	a.selectedJob = -1
	a.fileList.UnselectAll()
	a.btnRemoveJob.Disable()
	a.fileList.Refresh()

	var doneCount, totalCount int
	var doneBytes, totalBytes int64
	for _, job := range a.jobs {
		totalCount++
		totalBytes += job.Size
		if job.Status == statusDone {
			doneCount++
			doneBytes += job.Size
		}
	}
	a.updateOverall(doneCount, totalCount, doneBytes, totalBytes)
}

func (a *app_) applySort(jobs []*copyJob) {
	sort.Slice(jobs, func(i, j int) bool {
		switch a.sortOrder {
		case sortNameAsc:
			return naturalCompare(jobs[i].RelPath, jobs[j].RelPath) < 0
		case sortNameDesc:
			return naturalCompare(jobs[i].RelPath, jobs[j].RelPath) > 0
		case sortSizeAsc:
			if jobs[i].Size == jobs[j].Size {
				return naturalCompare(jobs[i].RelPath, jobs[j].RelPath) < 0
			}
			return jobs[i].Size < jobs[j].Size
		case sortSizeDesc:
			if jobs[i].Size == jobs[j].Size {
				return naturalCompare(jobs[i].RelPath, jobs[j].RelPath) < 0
			}
			return jobs[i].Size > jobs[j].Size
		}
		return naturalCompare(jobs[i].RelPath, jobs[j].RelPath) < 0
	})
}

func naturalCompare(left, right string) int {
	leftLower, rightLower := strings.ToLower(left), strings.ToLower(right)
	i, j := 0, 0
	for i < len(leftLower) && j < len(rightLower) {
		leftChar, rightChar := leftLower[i], rightLower[j]
		if leftChar >= '0' && leftChar <= '9' && rightChar >= '0' && rightChar <= '9' {
			leftEnd, rightEnd := i, j
			for leftEnd < len(leftLower) && leftLower[leftEnd] >= '0' && leftLower[leftEnd] <= '9' {
				leftEnd++
			}
			for rightEnd < len(rightLower) && rightLower[rightEnd] >= '0' && rightLower[rightEnd] <= '9' {
				rightEnd++
			}

			leftNumber, rightNumber := i, j
			for leftNumber < leftEnd && leftLower[leftNumber] == '0' {
				leftNumber++
			}
			for rightNumber < rightEnd && rightLower[rightNumber] == '0' {
				rightNumber++
			}
			leftDigits, rightDigits := leftLower[leftNumber:leftEnd], rightLower[rightNumber:rightEnd]
			if len(leftDigits) != len(rightDigits) {
				if len(leftDigits) < len(rightDigits) {
					return -1
				}
				return 1
			}
			if leftDigits != rightDigits {
				if leftDigits < rightDigits {
					return -1
				}
				return 1
			}
			i, j = leftEnd, rightEnd
			continue
		}

		if leftChar != rightChar {
			if leftChar < rightChar {
				return -1
			}
			return 1
		}
		i++
		j++
	}

	if i == len(leftLower) && j != len(rightLower) {
		return -1
	}
	if j == len(rightLower) && i != len(leftLower) {
		return 1
	}
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func (a *app_) updateOverall(doneCount, totalCount int, doneBytes, totalBytes int64) {
	fyne.Do(func() {
		a.overallLabel.SetText(fmt.Sprintf("%d / %d ไฟล์  (%s / %s)",
			doneCount, totalCount, humanSize(doneBytes), humanSize(totalBytes)))
		if totalBytes > 0 {
			a.overallProg.SetValue(float64(doneBytes) / float64(totalBytes))
		} else {
			a.overallProg.SetValue(0)
		}
	})
}

func humanSize(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func formatDuration(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
