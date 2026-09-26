package main

import "sync"

type jobStatus int

const (
	statusWaiting jobStatus = iota
	statusCopying
	statusVerifying
	statusRetrying
	statusDone
	statusSkipped
	statusError
	statusVerifyFailed
)

type copyJob struct {
	SrcPath string
	RelPath string
	Size    int64
	Status  jobStatus
	Err     error
	Retries int
}

func (j jobStatus) String() string {
	switch j {
	case statusWaiting:
		return "รอคิว"
	case statusCopying:
		return "กำลังคัดลอก"
	case statusVerifying:
		return "กำลังตรวจ Hash"
	case statusRetrying:
		return "กำลัง Retry..."
	case statusDone:
		return "เสร็จแล้ว"
	case statusSkipped:
		return "ข้าม"
	case statusError:
		return "ผิดพลาด"
	case statusVerifyFailed:
		return "Checksum ล้มเหลว!"
	}
	return ""
}

type overwritePolicy int

const (
	overwriteAlways overwritePolicy = iota
	overwriteAsk
	overwriteSkip
	overwriteRename
)

func (o overwritePolicy) String() string {
	switch o {
	case overwriteAlways:
		return "เขียนทับเสมอ (Overwrite)"
	case overwriteAsk:
		return "ถามก่อนเขียนทับ (Ask)"
	case overwriteSkip:
		return "ข้ามเมื่อเจอไฟล์ซ้ำ (Skip)"
	case overwriteRename:
		return "เปลี่ยนชื่ออัตโนมัติ (Rename)"
	}
	return ""
}

type verifyMode int

const (
	verifyNone verifyMode = iota
	verifyMD5
	verifySHA256
)

func (v verifyMode) String() string {
	switch v {
	case verifyNone:
		return "ไม่ตรวจสอบ (None)"
	case verifyMD5:
		return "ตรวจสอบ MD5 Hash"
	case verifySHA256:
		return "ตรวจสอบ SHA256 Hash"
	}
	return ""
}

type queueSortOrder int

const (
	sortNameAsc queueSortOrder = iota
	sortNameDesc
	sortSizeAsc
	sortSizeDesc
)

func (q queueSortOrder) String() string {
	switch q {
	case sortNameAsc:
		return "เรียงตามชื่อ (A-Z)"
	case sortNameDesc:
		return "เรียงตามชื่อ (Z-A)"
	case sortSizeAsc:
		return "เรียงตามขนาด (เล็ก -> ใหญ่)"
	case sortSizeDesc:
		return "เรียงตามขนาด (ใหญ่ -> เล็ก)"
	}
	return ""
}

type controller struct {
	mu        sync.Mutex
	cond      *sync.Cond
	paused    bool
	cancelled bool
}

func newController() *controller {
	c := &controller{}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *controller) togglePause() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = !c.paused
	c.cond.Broadcast()
	return c.paused
}

func (c *controller) cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelled = true
	c.paused = false
	c.cond.Broadcast()
}

func (c *controller) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = false
	c.cancelled = false
}

func (c *controller) waitIfPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.paused && !c.cancelled {
		c.cond.Wait()
	}
	return c.cancelled
}

func (c *controller) isCancelled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled
}
