package x11

import (
	"fmt"
	"strings"
	"unicode"

	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/test"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
)

// X11 event type constants
const (
	KeyPress         = 2
	KeyRelease       = 3
	ButtonPress      = 4
	ButtonRelease    = 5
	MotionNotify     = 6
)

// Wait pauses for the specified number of milliseconds
func (c *Client) Wait(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// MouseMove moves the mouse cursor to the specified coordinates
func (c *Client) MouseMove(x, y int) error {
	// Use XTEST to move mouse
	test.FakeInput(c.conn, MotionNotify, 0,
		0, // time (0 = current time)
		c.root, int16(x), int16(y), 0)
	return nil
}

// MouseClick simulates a mouse button click
func (c *Client) MouseClick(button int) error {
	// Press and release the button
	// Button press
	test.FakeInput(c.conn, ButtonPress, byte(button),
		0, // time
		c.root, 0, 0, 0)

	// Button release
	test.FakeInput(c.conn, ButtonRelease, byte(button),
		0, // time
		c.root, 0, 0, 0)

	return nil
}

// Type simulates typing the given text
func (c *Client) Type(text string) error {
	for _, ch := range text {
		// Handle newline as Enter key
		if ch == '\n' {
			if err := c.KeyPress("Enter"); err != nil {
				return fmt.Errorf("failed to press Enter key: %w", err)
			}
		} else {
			if err := c.typeChar(ch); err != nil {
				return fmt.Errorf("failed to type character '%c': %w", ch, err)
			}
		}
	}
	return nil
}

// typeChar types a single character
func (c *Client) typeChar(ch rune) error {
	// Get keysym for the character
	var keysym x.Keysym
	var needShift bool

	// Handle special characters that need shift
	switch ch {
	case '!': keysym, needShift = keysyms.XK_1, true
	case '@': keysym, needShift = keysyms.XK_2, true
	case '#': keysym, needShift = keysyms.XK_3, true
	case '$': keysym, needShift = keysyms.XK_4, true
	case '%': keysym, needShift = keysyms.XK_5, true
	case '^': keysym, needShift = keysyms.XK_6, true
	case '&': keysym, needShift = keysyms.XK_7, true
	case '*': keysym, needShift = keysyms.XK_8, true
	case '(': keysym, needShift = keysyms.XK_9, true
	case ')': keysym, needShift = keysyms.XK_0, true
	default:
		// For regular characters
		if unicode.IsUpper(ch) {
			needShift = true
			keysym = x.Keysym(unicode.ToLower(ch))
		} else {
			keysym = x.Keysym(ch)
		}
	}

	// Get keycode for the keysym
	keycode, err := c.keysymToKeycode(keysym)
	if err != nil {
		return err
	}

	// Press shift if needed
	if needShift {
		shiftKeycode, _ := c.keysymToKeycode(keysyms.XK_Shift_L)
		test.FakeInput(c.conn, KeyPress, uint8(shiftKeycode),
			0, c.root, 0, 0, 0)
	}

	// Press and release the key
	test.FakeInput(c.conn, KeyPress, uint8(keycode),
		0, c.root, 0, 0, 0)

	test.FakeInput(c.conn, KeyRelease, uint8(keycode),
		0, c.root, 0, 0, 0)

	// Release shift if it was pressed
	if needShift {
		shiftKeycode, _ := c.keysymToKeycode(keysyms.XK_Shift_L)
		test.FakeInput(c.conn, KeyRelease, uint8(shiftKeycode),
			0, c.root, 0, 0, 0)
	}

	return nil
}

// KeyPress simulates pressing a special key
func (c *Client) KeyPress(key string) error {
	keysym, err := c.keyNameToKeysym(key)
	if err != nil {
		return err
	}

	keycode, err := c.keysymToKeycode(keysym)
	if err != nil {
		return err
	}

	// Press and release the key
	test.FakeInput(c.conn, KeyPress, uint8(keycode),
		0, c.root, 0, 0, 0)

	test.FakeInput(c.conn, KeyRelease, uint8(keycode),
		0, c.root, 0, 0, 0)

	return nil
}

// KeyCombo simulates a key combination like "ctrl+c"
func (c *Client) KeyCombo(combo string) error {
	parts := strings.Split(strings.ToLower(combo), "+")
	if len(parts) < 2 {
		return fmt.Errorf("invalid key combo: %s", combo)
	}

	var modifiers []x.Keysym
	var mainKey string

	// Separate modifiers from main key
	for i, part := range parts {
		if i == len(parts)-1 {
			mainKey = part
		} else {
			switch part {
			case "ctrl":
				modifiers = append(modifiers, keysyms.XK_Control_L)
			case "shift":
				modifiers = append(modifiers, keysyms.XK_Shift_L)
			case "alt":
				modifiers = append(modifiers, keysyms.XK_Alt_L)
			case "super", "win", "cmd":
				modifiers = append(modifiers, keysyms.XK_Super_L)
			default:
				return fmt.Errorf("unknown modifier: %s", part)
			}
		}
	}

	// Press all modifiers
	for _, mod := range modifiers {
		keycode, err := c.keysymToKeycode(mod)
		if err != nil {
			return err
		}
		test.FakeInput(c.conn, KeyPress, uint8(keycode),
			0, c.root, 0, 0, 0)
	}

	// Press main key
	var mainKeysym x.Keysym
	if len(mainKey) == 1 {
		// Single character
		mainKeysym = x.Keysym(mainKey[0])
	} else {
		// Special key name
		var err error
		mainKeysym, err = c.keyNameToKeysym(mainKey)
		if err != nil {
			return err
		}
	}

	mainKeycode, err := c.keysymToKeycode(mainKeysym)
	if err != nil {
		return err
	}

	test.FakeInput(c.conn, KeyPress, uint8(mainKeycode),
		0, c.root, 0, 0, 0)

	// Release main key
	test.FakeInput(c.conn, KeyRelease, uint8(mainKeycode),
		0, c.root, 0, 0, 0)

	// Release all modifiers in reverse order
	for i := len(modifiers) - 1; i >= 0; i-- {
		keycode, _ := c.keysymToKeycode(modifiers[i])
		test.FakeInput(c.conn, KeyRelease, uint8(keycode),
			0, c.root, 0, 0, 0)
	}

	return nil
}

// keyNameToKeysym converts a key name to a keysym
func (c *Client) keyNameToKeysym(name string) (x.Keysym, error) {
	switch name {
	case "Return", "Enter":
		return keysyms.XK_Return, nil
	case "Tab":
		return keysyms.XK_Tab, nil
	case "Escape", "Esc":
		return keysyms.XK_Escape, nil
	case "BackSpace", "Backspace":
		return keysyms.XK_BackSpace, nil
	case "Delete", "Del":
		return keysyms.XK_Delete, nil
	case "Home":
		return keysyms.XK_Home, nil
	case "End":
		return keysyms.XK_End, nil
	case "Page_Up", "PageUp", "PgUp":
		return keysyms.XK_Page_Up, nil
	case "Page_Down", "PageDown", "PgDn":
		return keysyms.XK_Page_Down, nil
	case "Left":
		return keysyms.XK_Left, nil
	case "Right":
		return keysyms.XK_Right, nil
	case "Up":
		return keysyms.XK_Up, nil
	case "Down":
		return keysyms.XK_Down, nil
	case "tab":
		return keysyms.XK_Tab, nil
	case "delete":
		return keysyms.XK_Delete, nil
	default:
		return 0, fmt.Errorf("unknown key name: %s", name)
	}
}

// keysymToKeycode converts a keysym to a keycode
func (c *Client) keysymToKeycode(keysym x.Keysym) (x.Keycode, error) {
	setup := c.conn.GetSetup()
	minKeycode := setup.MinKeycode
	maxKeycode := setup.MaxKeycode

	// Get keyboard mapping
	cookie := x.GetKeyboardMapping(c.conn, minKeycode, byte(maxKeycode-minKeycode+1))
	reply, err := cookie.Reply(c.conn)
	if err != nil {
		return 0, fmt.Errorf("failed to get keyboard mapping: %w", err)
	}

	keysymsPerKeycode := int(reply.KeysymsPerKeycode)

	// Search for the keysym in the mapping
	for keycode := minKeycode; keycode <= maxKeycode; keycode++ {
		for col := 0; col < keysymsPerKeycode; col++ {
			idx := int(keycode-minKeycode)*keysymsPerKeycode + col
			if idx < len(reply.Keysyms) && reply.Keysyms[idx] == keysym {
				return keycode, nil
			}
		}
	}

	return 0, fmt.Errorf("no keycode found for keysym %d", keysym)
}