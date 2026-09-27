// Copyright (c) 2026 Naawaakrit
// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
// seqcopy - โปรแกรมคัดลอกไฟล์ทีละไฟล์เรียงตามตัวอักษร (คล้าย TeraCopy)
// เขียนด้วย Go + Fyne สำหรับ Linux
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {

	a := app.NewWithID("com.nawakarit.nCopy")
	//a.Settings().SetTheme(&MyTheme{})
	//icon := loadIcon(64)
	//a.SetIcon(icon)

	w := a.NewWindow("nCopy - คัดลอกไฟล์เรียงตามตัวอักษร")
	w.Resize(fyne.NewSize(720, 640))
	//w.SetIcon(icon)

	ap := &app_{fyneApp: a, win: w, ctrl: newController(), preserveMetadata: true, maxRetry: 3}

	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		added := false
		for _, u := range uris {
			if u.Scheme() == "file" || u.Scheme() == "" {
				path := u.Path()
				if path != "" {
					ap.sources = append(ap.sources, path)
					added = true
				}
			}
		}
		if added {
			ap.sourceList.Refresh()
			ap.rebuildQueue()
		}
	})

	w.SetContent(ap.buildUI())
	w.ShowAndRun()
}
