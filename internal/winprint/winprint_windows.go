package winprint

import (
	"errors"
	"fmt"
	"image"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winspool = windows.NewLazySystemDLL("winspool.drv")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procEnumPrinters       = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinter  = winspool.NewProc("GetDefaultPrinterW")
	procOpenPrinter        = winspool.NewProc("OpenPrinterW")
	procClosePrinter       = winspool.NewProc("ClosePrinter")
	procDocumentProperties = winspool.NewProc("DocumentPropertiesW")

	procCreateDC          = gdi32.NewProc("CreateDCW")
	procResetDC           = gdi32.NewProc("ResetDCW")
	procDeleteDC          = gdi32.NewProc("DeleteDC")
	procStartDoc          = gdi32.NewProc("StartDocW")
	procEndDoc            = gdi32.NewProc("EndDoc")
	procAbortDoc          = gdi32.NewProc("AbortDoc")
	procStartPage         = gdi32.NewProc("StartPage")
	procEndPage           = gdi32.NewProc("EndPage")
	procGetDeviceCaps     = gdi32.NewProc("GetDeviceCaps")
	procSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	procSetBrushOrgEx     = gdi32.NewProc("SetBrushOrgEx")
	procStretchDIBits     = gdi32.NewProc("StretchDIBits")
)

const (
	printerEnumLocal       = 0x2
	printerEnumConnections = 0x4

	dmOutBuffer = 2
	dmInBuffer  = 8

	// DEVMODEW field offsets and flags.
	dmFieldsOffset      = 72
	dmOrientationOffset = 76
	dmDuplexOffset      = 94
	dmOrientation       = 0x1
	dmDuplex            = 0x1000
	orientPortrait      = 1
	orientLandscape     = 2

	horzRes        = 8
	vertRes        = 10
	logPixelsX     = 88
	logPixelsY     = 90
	physicalWidth  = 110
	physicalHeight = 111
	physicalOffX   = 112
	physicalOffY   = 113

	halftone     = 4
	dibRGBColors = 0
	srcCopy      = 0x00CC0020
	gdiError     = 0xFFFFFFFF
)

// maxDPI caps the resolution pages are rendered at; GDI stretches them up
// to printers finer than this.
const maxDPI = 600

type Duplex int16

const (
	OneSided  Duplex = 0
	LongEdge  Duplex = 2 // DMDUP_VERTICAL
	ShortEdge Duplex = 3 // DMDUP_HORIZONTAL
)

type printerInfo4 struct {
	name       *uint16
	server     *uint16
	attributes uint32
}

// Printers lists the local and connected printers, and the default one.
func Printers() (names []string, defaultName string, err error) {
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	var needed, count uint32
	procEnumPrinters.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if needed > 0 {
		// Words rather than bytes keep the records' pointers aligned.
		buf := make([]uint64, (needed+7)/8)
		r, _, e := procEnumPrinters.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
		if r == 0 {
			return nil, "", callError("EnumPrinters", e)
		}
		for _, info := range unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buf[0])), count) {
			names = append(names, windows.UTF16PtrToString(info.name))
		}
	}
	return names, defaultPrinter(), nil
}

func defaultPrinter() string {
	var n uint32
	procGetDefaultPrinter.Call(0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	if r, _, _ := procGetDefaultPrinter.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// Job is a document being spooled to a printer. Use it from one goroutine,
// locked to its thread.
type Job struct {
	hdc         uintptr
	devmode     []uint64
	orientation int16
	bgra        []byte
}

// Start begins a print job titled title on printer, or on the default
// printer when printer is empty.
func Start(printer, title string, duplex Duplex) (*Job, error) {
	if printer == "" {
		if printer = defaultPrinter(); printer == "" {
			return nil, errors.New("no default printer")
		}
	}
	devmode, err := printerDevmode(printer, duplex)
	if err != nil {
		return nil, err
	}
	driver, _ := windows.UTF16PtrFromString("WINSPOOL")
	name, err := windows.UTF16PtrFromString(printer)
	if err != nil {
		return nil, err
	}
	hdc, _, e := procCreateDC.Call(uintptr(unsafe.Pointer(driver)), uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&devmode[0])))
	if hdc == 0 {
		return nil, callError("CreateDC "+printer, e)
	}
	docName, err := windows.UTF16PtrFromString(title)
	if err != nil {
		docName, _ = windows.UTF16PtrFromString("document")
	}
	info := struct {
		size     int32
		docName  *uint16
		output   *uint16
		datatype *uint16
		flags    uint32
	}{docName: docName}
	info.size = int32(unsafe.Sizeof(info))
	if r, _, e := procStartDoc.Call(hdc, uintptr(unsafe.Pointer(&info))); int32(r) <= 0 {
		procDeleteDC.Call(hdc)
		return nil, callError("StartDoc", e)
	}
	j := &Job{hdc: hdc, devmode: devmode}
	j.orientation = *(*int16)(j.devmodeField(dmOrientationOffset))
	return j, nil
}

// printerDevmode is the printer's default settings with duplex applied.
func printerDevmode(printer string, duplex Duplex) ([]uint64, error) {
	name, err := windows.UTF16PtrFromString(printer)
	if err != nil {
		return nil, err
	}
	var handle windows.Handle
	if r, _, e := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0); r == 0 {
		return nil, callError("OpenPrinter "+printer, e)
	}
	defer procClosePrinter.Call(uintptr(handle))
	size, _, e := procDocumentProperties.Call(0, uintptr(handle), uintptr(unsafe.Pointer(name)), 0, 0, 0)
	if int32(size) <= 0 {
		return nil, callError("DocumentProperties", e)
	}
	devmode := make([]uint64, (uintptr(int32(size))+7)/8)
	base := unsafe.Pointer(&devmode[0])
	dm := uintptr(base)
	if r, _, e := procDocumentProperties.Call(0, uintptr(handle), uintptr(unsafe.Pointer(name)), dm, 0, dmOutBuffer); int32(r) < 0 {
		return nil, callError("DocumentProperties", e)
	}
	if duplex != OneSided {
		*(*uint32)(unsafe.Add(base, dmFieldsOffset)) |= dmDuplex
		*(*int16)(unsafe.Add(base, dmDuplexOffset)) = int16(duplex)
		if r, _, e := procDocumentProperties.Call(0, uintptr(handle), uintptr(unsafe.Pointer(name)), dm, dm, dmInBuffer|dmOutBuffer); int32(r) < 0 {
			return nil, callError("DocumentProperties", e)
		}
	}
	return devmode, nil
}

func (j *Job) devmodeField(offset uintptr) unsafe.Pointer {
	return unsafe.Add(unsafe.Pointer(&j.devmode[0]), offset)
}

// End finishes the job, handing it to the spooler.
func (j *Job) End() error {
	r, _, e := procEndDoc.Call(j.hdc)
	procDeleteDC.Call(j.hdc)
	if int32(r) <= 0 {
		return callError("EndDoc", e)
	}
	return nil
}

// Abort cancels the job.
func (j *Job) Abort() {
	procAbortDoc.Call(j.hdc)
	procDeleteDC.Call(j.hdc)
}

// Page is a page being drawn; Scale is the render scale from points to the
// pixels Draw takes.
type Page struct {
	job        *Job
	Scale      float64
	x, y, w, h int32
}

// StartPage begins a page width×height points in size, turning the paper to
// match its orientation, and fits it to the printable area, shrinking it if
// it is too large, centred on the paper.
func (j *Job) StartPage(width, height float64) (*Page, error) {
	want := int16(orientPortrait)
	if width > height {
		want = orientLandscape
	}
	if want != j.orientation {
		*(*uint32)(j.devmodeField(dmFieldsOffset)) |= dmOrientation
		*(*int16)(j.devmodeField(dmOrientationOffset)) = want
		// A driver that cannot turn the paper keeps printing unturned.
		if r, _, _ := procResetDC.Call(j.hdc, uintptr(unsafe.Pointer(&j.devmode[0]))); r != 0 {
			j.orientation = want
		}
	}
	if r, _, e := procStartPage.Call(j.hdc); int32(r) <= 0 {
		return nil, callError("StartPage", e)
	}
	caps := func(index int) float64 {
		r, _, _ := procGetDeviceCaps.Call(j.hdc, uintptr(index))
		return float64(int32(r))
	}
	dpiX, dpiY := caps(logPixelsX), caps(logPixelsY)
	areaW, areaH := caps(horzRes), caps(vertRes)
	if dpiX <= 0 || dpiY <= 0 || areaW <= 0 || areaH <= 0 || width <= 0 || height <= 0 {
		return nil, errors.New("printer reports no printable area")
	}
	pageW, pageH := width*dpiX/72, height*dpiY/72
	fit := min(1, areaW/pageW, areaH/pageH)
	w, h := math.Round(pageW*fit), math.Round(pageH*fit)
	x := math.Max(0, math.Min((caps(physicalWidth)-w)/2-caps(physicalOffX), areaW-w))
	y := math.Max(0, math.Min((caps(physicalHeight)-h)/2-caps(physicalOffY), areaH-h))
	procSetStretchBltMode.Call(j.hdc, halftone)
	procSetBrushOrgEx.Call(j.hdc, 0, 0, 0)
	return &Page{
		job:   j,
		Scale: fit * min(dpiX, maxDPI) / 72,
		x:     int32(x), y: int32(y), w: int32(w), h: int32(h),
	}, nil
}

// Draw prints a band of the page rendered at Scale: rows top onward of a
// page rows pixels tall.
func (p *Page) Draw(band *image.RGBA, top, rows int) error {
	bw, bh := band.Rect.Dx(), band.Rect.Dy()
	if bw <= 0 || bh <= 0 || rows <= 0 {
		return nil
	}
	j := p.job
	if need := bw * bh * 4; cap(j.bgra) < need {
		j.bgra = make([]byte, need)
	}
	bgra := j.bgra[:bw*bh*4]
	for y := range bh {
		src := band.Pix[y*band.Stride : y*band.Stride+bw*4]
		dst := bgra[y*bw*4 : (y+1)*bw*4]
		for i := 0; i < len(src); i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], 0
		}
	}
	header := struct {
		size            uint32
		width, height   int32
		planes, bits    uint16
		compression     uint32
		sizeImage       uint32
		xPPM, yPPM      int32
		used, important uint32
		colors          [1]uint32
	}{width: int32(bw), height: -int32(bh), planes: 1, bits: 32}
	header.size = 40
	y0 := p.y + int32(int64(top)*int64(p.h)/int64(rows))
	y1 := p.y + int32(int64(top+bh)*int64(p.h)/int64(rows))
	r, _, e := procStretchDIBits.Call(j.hdc,
		uintptr(p.x), uintptr(y0), uintptr(p.w), uintptr(y1-y0),
		0, 0, uintptr(bw), uintptr(bh),
		uintptr(unsafe.Pointer(&bgra[0])), uintptr(unsafe.Pointer(&header)),
		dibRGBColors, srcCopy)
	if r == 0 || uint32(r) == gdiError {
		return callError("StretchDIBits", e)
	}
	return nil
}

// End finishes the page.
func (p *Page) End() error {
	if r, _, e := procEndPage.Call(p.job.hdc); int32(r) <= 0 {
		return callError("EndPage", e)
	}
	return nil
}

func callError(call string, err error) error {
	if errno, ok := err.(windows.Errno); ok && errno != 0 {
		return fmt.Errorf("%s: %w", call, err)
	}
	return fmt.Errorf("%s failed", call)
}
