// Copyright (c) 2026 Naawaakrit
// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	//"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"
)

type app_ struct {
	fyneApp fyne.App
	win     fyne.Window

	sources          []string
	destDir          string
	policy           overwritePolicy
	verify           verifyMode
	sortOrder        queueSortOrder
	preserveMetadata bool
	maxRetry         int

	errorLog []string
	jobs     []*copyJob

	ctrl    *controller
	running bool

	selectedJob  int
	btnRemoveJob *widget.Button

	sourceList   *widget.List
	destLabel    *widget.Label
	fileList     *widget.List
	currentLabel *widget.Label
	fileProgress *widget.ProgressBar
	overallProg  *widget.ProgressBar
	overallLabel *widget.Label
	speedLabel   *widget.Label
	etaLabel     *widget.Label
	btnStart     *widget.Button
	btnPause     *widget.Button
	btnCancel    *widget.Button
}

func (a *app_) buildUI() fyne.CanvasObject {
	a.selectedJob = -1
	a.sourceList = widget.NewList(
		func() int { return len(a.sources) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(a.sources[i])
		},
	)
	a.sourceList.Resize(fyne.NewSize(680, 100))

	btnAddFiles := widget.NewButtonWithIcon("เลือกไฟล์...", nil, func() {
		paths, err := dialog.File().Title("เลือกไฟล์").Load()
		if err != nil || len(paths) == 0 {
			return
		}
		a.sources = append(a.sources, paths)
		a.sourceList.Refresh()
		a.rebuildQueue()
	})

	btnAddFolder := widget.NewButtonWithIcon("เลือกโฟลเดอร์...", nil, func() {
		path, err := dialog.Directory().Title("เลือกโฟลเดอร์").Browse()
		if err != nil || path == "" {
			return
		}
		a.sources = append(a.sources, path)
		a.sourceList.Refresh()
		a.rebuildQueue()
	})

	btnClear := widget.NewButtonWithIcon("ล้างรายการ", nil, func() {
		a.sources = nil
		a.jobs = nil
		a.selectedJob = -1
		a.fileList.UnselectAll()
		a.btnRemoveJob.Disable()
		a.sourceList.Refresh()
		a.fileList.Refresh()
		a.updateOverall(0, 0, 0, 0)
	})

	sourceButtons := container.NewHBox(btnAddFiles, btnAddFolder, btnClear)

	a.destLabel = widget.NewLabel("(ยังไม่ได้เลือกโฟลเดอร์ปลายทาง)")

	btnDest := widget.NewButtonWithIcon("เลือกปลายทาง...", nil, func() {
		path, err := dialog.Directory().Title("เลือกปลายทาง").Browse()
		if err != nil || path == "" {
			return
		}
		a.destDir = path
		a.destLabel.SetText(a.destDir)
	})

	destRow := container.NewBorder(nil, nil, nil, btnDest, a.destLabel)

	policySelect := widget.NewSelect([]string{
		overwriteAlways.String(),
		overwriteAsk.String(),
		overwriteSkip.String(),
		overwriteRename.String(),
	}, func(selected string) {
		switch selected {
		case overwriteAlways.String():
			a.policy = overwriteAlways
		case overwriteAsk.String():
			a.policy = overwriteAsk
		case overwriteSkip.String():
			a.policy = overwriteSkip
		case overwriteRename.String():
			a.policy = overwriteRename
		}
	})
	policySelect.SetSelected(overwriteAlways.String())

	verifySelect := widget.NewSelect([]string{
		verifyNone.String(),
		verifyMD5.String(),
		verifySHA256.String(),
	}, func(selected string) {
		switch selected {
		case verifyNone.String():
			a.verify = verifyNone
		case verifyMD5.String():
			a.verify = verifyMD5
		case verifySHA256.String():
			a.verify = verifySHA256
		}
	})
	verifySelect.SetSelected(verifyNone.String())

	sortSelect := widget.NewSelect([]string{
		sortNameAsc.String(),
		sortNameDesc.String(),
		sortSizeAsc.String(),
		sortSizeDesc.String(),
	}, func(selected string) {
		switch selected {
		case sortNameAsc.String():
			a.sortOrder = sortNameAsc
		case sortNameDesc.String():
			a.sortOrder = sortNameDesc
		case sortSizeAsc.String():
			a.sortOrder = sortSizeAsc
		case sortSizeDesc.String():
			a.sortOrder = sortSizeDesc
		}
		if len(a.jobs) > 0 {
			a.applySort(a.jobs)
			a.fileList.Refresh()
		}
	})
	sortSelect.SetSelected(sortNameAsc.String())

	preserveCheck := widget.NewCheck("คงค่า Metadata (เวลา/สิทธิ์ไฟล์)", func(checked bool) {
		a.preserveMetadata = checked
	})
	preserveCheck.SetChecked(a.preserveMetadata)

	retrySelect := widget.NewSelect([]string{"ไม่ Retry (0)", "1 ครั้ง", "3 ครั้ง", "5 ครั้ง", "10 ครั้ง"}, func(selected string) {
		switch selected {
		case "ไม่ Retry (0)":
			a.maxRetry = 0
		case "1 ครั้ง":
			a.maxRetry = 1
		case "3 ครั้ง":
			a.maxRetry = 3
		case "5 ครั้ง":
			a.maxRetry = 5
		case "10 ครั้ง":
			a.maxRetry = 10
		}
	})
	retrySelect.SetSelected("3 ครั้ง")

	optionsRow := container.NewHBox(
		widget.NewLabelWithStyle("กรณีไฟล์ซ้ำ:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		policySelect,
		widget.NewLabelWithStyle("  เรียงคิว:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		sortSelect,
		widget.NewLabelWithStyle("  ตรวจสอบ:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		verifySelect,
		preserveCheck,
	)

	retryRow := container.NewHBox(
		widget.NewLabelWithStyle("Retry เมื่อเกิด Error:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		retrySelect,
	)

	a.fileList = widget.NewList(
		func() int { return len(a.jobs) },
		func() fyne.CanvasObject {
			nameLbl := widget.NewLabel("")
			statusLbl := widget.NewLabel("")
			statusLbl.Alignment = fyne.TextAlignTrailing
			return container.NewHBox(nameLbl, statusLbl)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			nameLbl := row.Objects[0].(*widget.Label)
			statusLbl := row.Objects[1].(*widget.Label)
			j := a.jobs[i]
			nameLbl.SetText(j.RelPath)
			statusLbl.SetText(j.Status.String())
		},
	)
	a.fileList.OnSelected = func(id widget.ListItemID) {
		a.selectedJob = int(id)
		if !a.running {
			a.btnRemoveJob.Enable()
		}
	}
	a.fileList.OnUnselected = func(id widget.ListItemID) {
		if a.selectedJob == int(id) {
			a.selectedJob = -1
			a.btnRemoveJob.Disable()
		}
	}
	a.btnRemoveJob = widget.NewButton("ลบรายการที่เลือก", func() {
		a.removeSelectedJob()
	})
	a.btnRemoveJob.Disable()

	queueHeader := container.NewBorder(
		nil, nil,
		widget.NewLabelWithStyle("คิวไฟล์ (เรียงตามตัวอักษร)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		a.btnRemoveJob,
	)

	a.currentLabel = widget.NewLabel("ยังไม่เริ่มคัดลอก")
	a.fileProgress = widget.NewProgressBar()
	a.overallProg = widget.NewProgressBar()
	a.overallLabel = widget.NewLabel("0 / 0 ไฟล์  (0 B / 0 B)")
	a.speedLabel = widget.NewLabel("")
	a.etaLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	a.btnStart = widget.NewButtonWithIcon("เริ่มคัดลอก", nil, a.onStart)
	a.btnPause = widget.NewButtonWithIcon("หยุดชั่วคราว", nil, a.onPauseResume)
	a.btnCancel = widget.NewButtonWithIcon("ยกเลิก", nil, a.onCancel)
	a.btnPause.Disable()
	a.btnCancel.Disable()

	controlRow := container.NewHBox(a.btnStart, a.btnPause, a.btnCancel)

	progressBox := container.NewVBox(
		a.currentLabel,
		a.fileProgress,
		a.overallLabel,
		a.overallProg,
		container.NewHBox(a.speedLabel, widget.NewLabel("  "), a.etaLabel),
		controlRow,
	)

	top := container.NewVBox(
		widget.NewLabelWithStyle("ต้นฉบับ", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVScroll(a.sourceList),
		sourceButtons,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("ปลายทาง", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		destRow,
		optionsRow,
		retryRow,
		widget.NewSeparator(),
		queueHeader,
	)

	center := container.NewVScroll(a.fileList)
	center.SetMinSize(fyne.NewSize(680, 220))

	return container.NewBorder(top, progressBox, nil, nil, center)
}
