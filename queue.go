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
	a.fileList.Refresh()
	var total int64
	for _, j := range jobs {
		total += j.Size
	}
	a.updateOverall(0, len(jobs), 0, total)
}

func (a *app_) applySort(jobs []*copyJob) {
	sort.Slice(jobs, func(i, j int) bool {
		switch a.sortOrder {
		case sortNameAsc:
			return strings.ToLower(jobs[i].RelPath) < strings.ToLower(jobs[j].RelPath)
		case sortNameDesc:
			return strings.ToLower(jobs[i].RelPath) > strings.ToLower(jobs[j].RelPath)
		case sortSizeAsc:
			if jobs[i].Size == jobs[j].Size {
				return strings.ToLower(jobs[i].RelPath) < strings.ToLower(jobs[j].RelPath)
			}
			return jobs[i].Size < jobs[j].Size
		case sortSizeDesc:
			if jobs[i].Size == jobs[j].Size {
				return strings.ToLower(jobs[i].RelPath) < strings.ToLower(jobs[j].RelPath)
			}
			return jobs[i].Size > jobs[j].Size
		}
		return strings.ToLower(jobs[i].RelPath) < strings.ToLower(jobs[j].RelPath)
	})
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
