package computer

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// Linux input constants (linux/input-event-codes.h, linux/uinput.h).
const (
	evSyn, evKey, evRel, evAbs = 0, 1, 2, 3
	absX, absY                 = 0, 1
	relHWheel, relWheel        = 6, 8
	absMax                     = 65535

	uiSetEvBit   = 0x40045564
	uiSetKeyBit  = 0x40045565
	uiSetRelBit  = 0x40045566
	uiSetAbsBit  = 0x40045567
	uiDevSetup   = 0x405c5503
	uiAbsSetup   = 0x401c5504
	uiDevCreate  = 0x5501
	busVirtual   = 0x06
	vendorVibian = 0x5669
)

var buttons = map[string]uint16{"left": 0x110, "right": 0x111, "middle": 0x112}

// device is a virtual input device. Compositors take a moment to pick a new
// one up, so the MCP server keeps it for its whole life.
type device struct{ f *os.File }

func ioctl(fd, req uintptr, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}

func newDevice(name string, product uint16, setup func(fd uintptr) error) (*device, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("can't open /dev/uinput (%v): Vibian's udev rule gives it to whoever is logged in at this machine's screen; log in there (or, for a test, sudo chmod 666 /dev/uinput)", err)
	}
	fd := f.Fd()
	if err := setup(fd); err != nil {
		f.Close()
		return nil, err
	}
	var s [92]byte // struct uinput_setup
	binary.LittleEndian.PutUint16(s[0:], busVirtual)
	binary.LittleEndian.PutUint16(s[2:], vendorVibian)
	binary.LittleEndian.PutUint16(s[4:], product)
	binary.LittleEndian.PutUint16(s[6:], 1)
	copy(s[8:87], name)
	err = ioctl(fd, uiDevSetup, uintptr(unsafe.Pointer(&s[0])))
	runtime.KeepAlive(&s)
	if err == nil {
		err = ioctl(fd, uiDevCreate, 0)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("creating the uinput device: %v", err)
	}
	time.Sleep(800 * time.Millisecond) // let udev tag it and the compositor open it
	return &device{f}, nil
}

// emit writes events followed by a SYN_REPORT.
func (d *device) emit(evs ...[3]int32) error {
	buf := make([]byte, 0, 24*(len(evs)+1))
	for _, e := range append(evs, [3]int32{evSyn, 0, 0}) {
		var b [24]byte // struct input_event on 64-bit: timeval, type, code, value
		binary.LittleEndian.PutUint16(b[16:], uint16(e[0]))
		binary.LittleEndian.PutUint16(b[18:], uint16(e[1]))
		binary.LittleEndian.PutUint32(b[20:], uint32(e[2]))
		buf = append(buf, b[:]...)
	}
	_, err := d.f.Write(buf)
	return err
}

func setBits(fd uintptr, req uintptr, codes ...uint16) error {
	for _, c := range codes {
		if err := ioctl(fd, req, uintptr(c)); err != nil {
			return fmt.Errorf("uinput ioctl %#x %d: %v", req, c, err)
		}
	}
	return nil
}

// Tablet is a uinput pointer with absolute axes, buttons and a wheel, like a
// VM's USB tablet. Compositors map its axes onto the box around all monitors,
// so Move needs that box.
type Tablet struct {
	dev    *device
	bounds func() (Rect, error)
	x, y   int
	placed bool
}

// NewTablet creates the device. bounds is asked for the monitor layout on
// every move, so it follows monitors being plugged in.
func NewTablet(bounds func() (Rect, error)) (*Tablet, error) {
	d, err := newDevice("vibian-computer pointer", 1, func(fd uintptr) error {
		if err := setBits(fd, uiSetEvBit, evKey, evRel, evAbs); err != nil {
			return err
		}
		if err := setBits(fd, uiSetKeyBit, buttons["left"], buttons["right"], buttons["middle"]); err != nil {
			return err
		}
		if err := setBits(fd, uiSetRelBit, relWheel, relHWheel); err != nil {
			return err
		}
		for _, axis := range []uint16{absX, absY} {
			if err := setBits(fd, uiSetAbsBit, axis); err != nil {
				return err
			}
			var a [28]byte // struct uinput_abs_setup: code, pad, input_absinfo
			binary.LittleEndian.PutUint16(a[0:], axis)
			binary.LittleEndian.PutUint32(a[12:], absMax) // maximum; value, minimum = 0
			err := ioctl(fd, uiAbsSetup, uintptr(unsafe.Pointer(&a[0])))
			runtime.KeepAlive(&a)
			if err != nil {
				return fmt.Errorf("uinput abs setup: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Tablet{dev: d, bounds: bounds}, nil
}

// absValue maps a layout coordinate onto the axis. libinput hands
// compositors (value-min)/(max-min+1) of the box, so aim at the pixel centre.
func absValue(p, origin, size int) int32 {
	if size <= 0 {
		return 0
	}
	v := (float64(p-origin) + 0.5) * (absMax + 1) / float64(size)
	return int32(max(0, min(absMax, v)))
}

func (t *Tablet) Move(x, y int) error {
	b, err := t.bounds()
	if err != nil {
		return err
	}
	if b.Empty() {
		return fmt.Errorf("no monitors to move the pointer on")
	}
	if x < b.X || y < b.Y || x >= b.X+b.W || y >= b.Y+b.H {
		return fmt.Errorf("%d,%d is off screen (the screen is %d,%d to %d,%d)", x, y, b.X, b.Y, b.X+b.W-1, b.Y+b.H-1)
	}
	if err := t.dev.emit([3]int32{evAbs, absX, absValue(x, b.X, b.W)}, [3]int32{evAbs, absY, absValue(y, b.Y, b.H)}); err != nil {
		return err
	}
	t.x, t.y, t.placed = x, y, true
	time.Sleep(30 * time.Millisecond)
	return nil
}

func (t *Tablet) Button(name string, down bool) error {
	code, ok := buttons[name]
	if !ok {
		return fmt.Errorf("unknown button %q (left, right or middle)", name)
	}
	v := int32(0)
	if down {
		v = 1
	}
	return t.dev.emit([3]int32{evKey, int32(code), v})
}

func (t *Tablet) Scroll(dy, dx int) error {
	step := func(axis int32, n, sign int) error {
		for range abs(n) {
			if err := t.dev.emit([3]int32{evRel, axis, int32(sign)}); err != nil {
				return err
			}
			time.Sleep(30 * time.Millisecond)
		}
		return nil
	}
	// the wheel's "up" is positive, so scrolling down is negative
	if err := step(relWheel, dy, -sgn(dy)); err != nil {
		return err
	}
	return step(relHWheel, dx, sgn(dx))
}

func (t *Tablet) Position() (int, int, bool) { return t.x, t.y, t.placed }

func abs(n int) int { return max(n, -n) }

func sgn(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// Keyboard is a uinput keyboard with a US layout, for desktops without a
// virtual keyboard protocol (KDE, GNOME). It types ASCII only.
type Keyboard struct{ dev *device }

func NewKeyboard() (*Keyboard, error) {
	d, err := newDevice("vibian-computer keyboard", 2, func(fd uintptr) error {
		if err := setBits(fd, uiSetEvBit, evKey); err != nil {
			return err
		}
		for c := uint16(1); c < 200; c++ {
			if err := setBits(fd, uiSetKeyBit, c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Keyboard{d}, nil
}

func (k *Keyboard) press(codes ...uint16) error {
	for _, c := range codes {
		if err := k.dev.emit([3]int32{evKey, int32(c), 1}); err != nil {
			return err
		}
	}
	time.Sleep(10 * time.Millisecond)
	for i := len(codes) - 1; i >= 0; i-- {
		if err := k.dev.emit([3]int32{evKey, int32(codes[i]), 0}); err != nil {
			return err
		}
	}
	time.Sleep(8 * time.Millisecond)
	return nil
}

func (k *Keyboard) Type(text string) error {
	var seq [][]uint16
	for _, r := range text {
		code, shift, ok := charKey(r)
		if !ok {
			return fmt.Errorf("can't type %q here: this desktop's keyboard fallback is US ASCII only; paste it instead (wl-copy/xclip, then ctrl+v)", r)
		}
		if shift {
			seq = append(seq, []uint16{keyCodes["shift"], code})
		} else {
			seq = append(seq, []uint16{code})
		}
	}
	for _, s := range seq {
		if err := k.press(s...); err != nil {
			return err
		}
	}
	return nil
}

func (k *Keyboard) Key(combo string) error {
	codes, err := comboCodes(combo)
	if err != nil {
		return err
	}
	return k.press(codes...)
}
