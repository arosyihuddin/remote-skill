//go:build linux

package handlers

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ioctl commands for /dev/uinput
const (
	uiDevCreate  = 0x00005501  // _IO('U', 1)
	uiDevSetup   = 0x405c5503  // _IOW('U', 3, sizeof(struct uinput_setup)=92)
	uiSetEvBit   = 0x40045564  // _IOW('U', 100, sizeof(int)=4)
	uiSetKeyBit  = 0x40045565  // _IOW('U', 101, sizeof(int)=4)
	uiSetRelBit  = 0x40045566  // _IOW('U', 102, sizeof(int)=4)
	uiSetAbsBit  = 0x40045567  // _IOW('U', 103, sizeof(int)=4)
	uiSetPropBit = 0x4004556e  // _IOW('U', 110, sizeof(int)=4)
	uiAbsSetup   = 0x401c5504  // _IOW('U', 4, sizeof(uinput_abs_setup)=28)
)

// event types
const (
	evKey = 0x01
	evRel = 0x02
	evAbs = 0x03
	evSyn = 0x00
)

const synReport = 0

// relative axes
const (
	relX     = 0x00
	relY     = 0x01
	relWheel = 0x08
)

// absolute axes
const (
	absX = 0x00
	absY = 0x01
)

// mouse buttons
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// key codes
const (
	keyLeftCtrl  = 29
	keyLeftShift = 42
	keyLeftAlt   = 56
	keyLeftMeta  = 125
	keySpace     = 57
	keyEnter     = 28
	keyTab       = 15
	keyEsc       = 1
	keyBackspace = 14
	keyDelete    = 111
	keyHome      = 102
	keyEnd       = 107
	keyInsert    = 110
	keyPageUp    = 104
	keyPageDown  = 109
	keyUp        = 103
	keyDown      = 108
	keyLeft      = 105
	keyRight     = 106
	keyCapsLock  = 58
)

type inputID struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

type uinputSetup struct {
	ID            inputID
	Name          [80]byte
	FfEffectsMax  uint32
}

type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

type inputAbsinfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

type uinputAbsSetup struct {
	Code    uint16
	_       [2]byte
	Absinfo inputAbsinfo
}

type uinputDevice struct {
	fd   int
	mu   sync.Mutex
	name string
}

var (
	kbdDev   *uinputDevice
	mouseDev  *uinputDevice
	kbdMu    sync.Mutex
	mouseMu  sync.Mutex
)

func ioctl(fd int, op uintptr, arg uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), op, arg)
	if err != 0 {
		return fmt.Errorf("ioctl %x: %w", op, err)
	}
	return nil
}

func setAbsAxis(fd int, code uint16, min, max int32) error {
	s := uinputAbsSetup{
		Code: code,
		Absinfo: inputAbsinfo{
			Minimum: min,
			Maximum: max,
		},
	}
	return ioctl(fd, uiAbsSetup, uintptr(unsafe.Pointer(&s)))
}

func openUinput(name string, evTypes []uint16, keyCodes []uint16, relAxes []uint16, absAxes []uint16, absMin, absMax []int32, props []uint16) (*uinputDevice, error) {
	fd, err := syscall.Open("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/uinput: %w", err)
	}

	for _, t := range evTypes {
		if err := ioctl(fd, uiSetEvBit, uintptr(t)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("set evbit %d: %w", t, err)
		}
	}
	for _, p := range props {
		if err := ioctl(fd, uiSetPropBit, uintptr(p)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("set propbit %d: %w", p, err)
		}
	}
	for _, k := range keyCodes {
		if err := ioctl(fd, uiSetKeyBit, uintptr(k)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("set keybit %d: %w", k, err)
		}
	}
	for _, a := range relAxes {
		if err := ioctl(fd, uiSetRelBit, uintptr(a)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("set relbit %d: %w", a, err)
		}
	}
	for i, a := range absAxes {
		if err := ioctl(fd, uiSetAbsBit, uintptr(a)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("set absbit %d: %w", a, err)
		}
		if len(absMin) > i && len(absMax) > i {
			if err := setAbsAxis(fd, a, absMin[i], absMax[i]); err != nil {
				syscall.Close(fd)
				return nil, fmt.Errorf("set absaxis %d: %w", a, err)
			}
		}
	}

	setup := uinputSetup{
		ID: inputID{
			Bustype: 0x03, // BUS_USB
			Vendor:  0x1234,
			Product: 0x5678,
			Version: 1,
		},
		FfEffectsMax: 0,
	}
	copy(setup.Name[:], name)

	if err := ioctl(fd, uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("dev setup: %w", err)
	}
	if err := ioctl(fd, uiDevCreate, 0); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("dev create: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	return &uinputDevice{fd: fd, name: name}, nil
}

func (d *uinputDevice) sendEvent(evType, code uint16, value int32) error {
	var ts syscall.Timeval
	syscall.Gettimeofday(&ts)

	ev := inputEvent{
		Sec:   ts.Sec,
		Usec:  ts.Usec,
		Type:  evType,
		Code:  code,
		Value: value,
	}

	var buf [24]byte
	*(*inputEvent)(unsafe.Pointer(&buf[0])) = ev

	_, err := syscall.Write(d.fd, buf[:])
	return err
}

func (d *uinputDevice) sync() error {
	return d.sendEvent(evSyn, synReport, 0)
}

func (d *uinputDevice) keyDown(code uint16) error {
	if err := d.sendEvent(evKey, code, 1); err != nil {
		return err
	}
	return d.sync()
}

func (d *uinputDevice) keyUp(code uint16) error {
	if err := d.sendEvent(evKey, code, 0); err != nil {
		return err
	}
	return d.sync()
}

var kbdKeys []uint16

func init() {
	kbdKeys = make([]uint16, 0, 768)
	for i := 0; i < 768; i++ {
		kbdKeys = append(kbdKeys, uint16(i))
	}
}

func getKeyboard() (*uinputDevice, error) {
	kbdMu.Lock()
	defer kbdMu.Unlock()
	if kbdDev != nil {
		return kbdDev, nil
	}
	dev, err := openUinput("remote-skill-kbd",
		[]uint16{evKey},
		kbdKeys,
		nil, nil, nil, nil, nil,
	)
	if err != nil {
		return nil, err
	}
	kbdDev = dev
	return kbdDev, nil
}

func getMouse() (*uinputDevice, error) {
	mouseMu.Lock()
	defer mouseMu.Unlock()
	if mouseDev != nil {
		return mouseDev, nil
	}
	absAxes := []uint16{0x00, 0x01}
	absMin := []int32{0, 0}
	absMax := []int32{65535, 65535}
	dev, err := openUinput("remote-skill-mouse",
		[]uint16{evKey, evRel, evAbs},
		[]uint16{btnLeft, btnRight, btnMiddle},
		[]uint16{relX, relY, relWheel},
		absAxes, absMin, absMax,
		[]uint16{0}, // INPUT_PROP_POINTER
	)
	if err != nil {
		return nil, err
	}
	mouseDev = dev
	return mouseDev, nil
}

func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// key name -> key code
var keyNameMap = map[string]uint16{
	"ctrl":     keyLeftCtrl,
	"control":  keyLeftCtrl,
	"alt":      keyLeftAlt,
	"shift":    keyLeftShift,
	"super":    keyLeftMeta,
	"meta":     keyLeftMeta,
	"cmd":      keyLeftMeta,
	"win":      keyLeftMeta,
	"enter":    keyEnter,
	"return":   keyEnter,
	"tab":      keyTab,
	"space":    keySpace,
	"escape":   keyEsc,
	"esc":      keyEsc,
	"backspace": keyBackspace,
	"bs":       keyBackspace,
	"delete":   keyDelete,
	"del":      keyDelete,
	"home":     keyHome,
	"end":      keyEnd,
	"insert":   keyInsert,
	"ins":      keyInsert,
	"pageup":   keyPageUp,
	"pgup":     keyPageUp,
	"pagedown": keyPageDown,
	"pgdn":     keyPageDown,
	"pgdown":   keyPageDown,
	"up":       keyUp,
	"down":     keyDown,
	"left":     keyLeft,
	"right":    keyRight,
	"capslock": keyCapsLock,
	"f1":       59, "f2": 60, "f3": 61, "f4": 62,
	"f5": 63, "f6": 64, "f7": 65, "f8": 66,
	"f9": 67, "f10": 68, "f11": 87, "f12": 88,
}

// ASCII char -> keycode + shift
var charKeyMap = map[rune]struct {
	code  uint16
	shift bool
}{
	'a': {30, false}, 'b': {48, false}, 'c': {46, false}, 'd': {32, false},
	'e': {18, false}, 'f': {33, false}, 'g': {34, false}, 'h': {35, false},
	'i': {23, false}, 'j': {36, false}, 'k': {37, false}, 'l': {38, false},
	'm': {50, false}, 'n': {49, false}, 'o': {24, false}, 'p': {25, false},
	'q': {16, false}, 'r': {19, false}, 's': {31, false}, 't': {20, false},
	'u': {22, false}, 'v': {47, false}, 'w': {17, false}, 'x': {45, false},
	'y': {21, false}, 'z': {44, false},
	'A': {30, true}, 'B': {48, true}, 'C': {46, true}, 'D': {32, true},
	'E': {18, true}, 'F': {33, true}, 'G': {34, true}, 'H': {35, true},
	'I': {23, true}, 'J': {36, true}, 'K': {37, true}, 'L': {38, true},
	'M': {50, true}, 'N': {49, true}, 'O': {24, true}, 'P': {25, true},
	'Q': {16, true}, 'R': {19, true}, 'S': {31, true}, 'T': {20, true},
	'U': {22, true}, 'V': {47, true}, 'W': {17, true}, 'X': {45, true},
	'Y': {21, true}, 'Z': {44, true},
	'1': {2, false}, '2': {3, false}, '3': {4, false}, '4': {5, false},
	'5': {6, false}, '6': {7, false}, '7': {8, false}, '8': {9, false},
	'9': {10, false}, '0': {11, false},
	'!': {2, true}, '@': {3, true}, '#': {4, true}, '$': {5, true},
	'%': {6, true}, '^': {7, true}, '&': {8, true}, '*': {9, true},
	'(': {10, true}, ')': {11, true},
	'-': {12, false}, '_': {12, true}, '=': {13, false}, '+': {13, true},
	'[': {26, false}, '{': {26, true}, ']': {27, false}, '}': {27, true},
	'\\': {43, false}, '|': {43, true},
	';': {39, false}, ':': {39, true},
	'\'': {40, false}, '"': {40, true},
	',': {51, false}, '<': {51, true}, '.': {52, false}, '>': {52, true},
	'/': {53, false}, '?': {53, true},
	'`': {41, false}, '~': {41, true},
	' ': {57, false}, '\t': {15, false}, '\n': {28, false},
}

// --- platform input functions called by gui.go ---

func platformMouseClick(btn string, double bool) error {
	m, err := getMouse()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var code uint16
	switch btn {
	case "right":
		code = btnRight
	case "middle":
		code = btnMiddle
	default:
		code = btnLeft
	}

	if double {
		m.sendEvent(evKey, code, 1)
		m.sync()
		sleep(20)
		m.sendEvent(evKey, code, 0)
		m.sync()
		sleep(20)
	}
	m.sendEvent(evKey, code, 1)
	m.sync()
	sleep(20)
	m.sendEvent(evKey, code, 0)
	m.sync()
	return nil
}

func platformMoveMouse(x, y int) error {
	m, err := getMouse()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendEvent(evAbs, 0x00, int32(x))
	m.sendEvent(evAbs, 0x01, int32(y))
	return m.sync()
}

func platformMoveMouseRel(x, y int) error {
	m, err := getMouse()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendEvent(evRel, relX, int32(x))
	m.sendEvent(evRel, relY, int32(y))
	return m.sync()
}

func platformMouseScroll(dy int) error {
	m, err := getMouse()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendEvent(evRel, relWheel, int32(dy))
	return m.sync()
}

func platformTypeText(text string) error {
	k, err := getKeyboard()
	if err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, ch := range text {
		m, ok := charKeyMap[ch]
		if !ok {
			continue
		}
		if m.shift {
			k.sendEvent(evKey, keyLeftShift, 1)
			k.sync()
		}
		k.sendEvent(evKey, m.code, 1)
		k.sync()
		sleep(20)
		k.sendEvent(evKey, m.code, 0)
		k.sync()
		if m.shift {
			k.sendEvent(evKey, keyLeftShift, 0)
			k.sync()
		}
		sleep(15)
	}
	return nil
}

func platformKeyCombo(parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty key combo")
	}
	k, err := getKeyboard()
	if err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	mods := parts[:len(parts)-1]
	key := parts[len(parts)-1]

	for _, m := range mods {
		code, ok := keyNameMap[m]
		if !ok {
			return fmt.Errorf("unknown modifier: %s", m)
		}
		k.sendEvent(evKey, code, 1)
	}
	k.sync()

	code, ok := keyNameMap[key]
	if !ok {
		char, ok2 := charKeyMap[[]rune(key)[0]]
		if !ok2 {
			return fmt.Errorf("unknown key: %s", key)
		}
		code = char.code
	}
	k.sendEvent(evKey, code, 1)
	k.sync()
	sleep(20)
	k.sendEvent(evKey, code, 0)
	k.sync()

	for i := len(mods) - 1; i >= 0; i-- {
		code := keyNameMap[mods[i]]
		k.sendEvent(evKey, code, 0)
	}
	k.sync()
	return nil
}
