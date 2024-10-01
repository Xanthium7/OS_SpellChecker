package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/lxn/win"
	"github.com/sahilm/fuzzy"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getClipboardData = user32.NewProc("GetClipboardData")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
)

const (
	MOD_ALT  = 0x0001
	MOD_CTRL = 0x0002
	VK_S     = 0x53 // Virtual key code for 'S'
)

var dictionary []string

func loadDictionary(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open dictionary file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dictionary = append(dictionary, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read dictionary file: %v", err)
	}
}
func main() {
	loadDictionary("dictionary.txt")
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("Spell Checker")
	systray.SetTooltip("Copy text, then click here to check spelling")
	mSpellCheck := systray.AddMenuItem("Check Clipboard Spelling", "Check spelling of clipboard text")

	go func() {
		for {
			select {
			case <-mSpellCheck.ClickedCh:
				checkSpelling()
			}
		}
	}()
}

func onExit() {
	// Cleanup
}

func checkSpelling() {
	text := getClipboardText()
	if text == "" {
		return
	}

	correctedText := correctSpelling(text)
	setClipboardText(correctedText)
}

func correctSpelling(text string) string {
	words := strings.Fields(text)
	var correctedWords []string

	for _, word := range words {
		correctedWord := findClosestMatch(strings.ToLower(word), dictionary)
		if correctedWord != "" {
			correctedWords = append(correctedWords, correctedWord)
		} else {
			correctedWords = append(correctedWords, word)
		}
	}

	return strings.Join(correctedWords, " ")
}

func findClosestMatch(word string, dictionary []string) string {
	matches := fuzzy.Find(word, dictionary)
	if len(matches) > 0 {
		return matches[0].Str
	}
	return ""
}

func getClipboardText() string {
	openClipboard.Call(0)
	defer closeClipboard.Call()

	h, _, _ := getClipboardData.Call(win.CF_UNICODETEXT)
	if h == 0 {
		return ""
	}

	p := win.GlobalLock(win.HGLOBAL(h))
	defer win.GlobalUnlock(win.HGLOBAL(h))

	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
}

func setClipboardText(text string) {
	openClipboard.Call(0)
	defer closeClipboard.Call()

	emptyClipboard.Call()

	utf16, _ := syscall.UTF16FromString(text)
	h := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(len(utf16)*2))
	p := win.GlobalLock(h)

	copy((*[1 << 20]uint16)(unsafe.Pointer(p))[:], utf16)

	win.GlobalUnlock(h)
	setClipboardData.Call(win.CF_UNICODETEXT, uintptr(h))
}
