package main

import (
	"encoding/hex"
	"os"

	"runtime/volatile"
)

var outbuf [80]byte

// printHWAddr formats a MAC address using ':' separator
func printHWAddr(hw []byte) {
	for i, b := range hw {
		if i > 0 {
			os.Stdout.Write([]byte(":"))
		}
		hex.Encode(outbuf[:2], []byte{b})
		os.Stdout.Write(outbuf[:2])
	}
	os.Stdout.Write([]byte("\n"))
}

// dumpFrame prints a frame to stdout in dense hex format with groups
func dumpFrame(frame []byte) {
	// Header summary
	os.Stdout.Write([]byte("Source:      "))
	printHWAddr(frame[6:12])
	os.Stdout.Write([]byte("Destination: "))
	printHWAddr(frame[0:6])

	os.Stdout.Write([]byte("EtherType: 0x"))
	etherType := uint16(frame[12])<<8 | uint16(frame[13])
	hex.Encode(outbuf[:4], []byte{byte(etherType >> 8), byte(etherType)})
	os.Stdout.Write(outbuf[:4])
	os.Stdout.Write([]byte("\n\nData:\n"))

	dumpHex(frame[14:])
}

func dumpHex(data []byte) {
	// Hex dump
	for i := 0; i < len(data); i += 32 {
		end := i + 32
		if end > len(data) {
			end = len(data)
		}

		// Prefix offset
		n := hex.Encode(outbuf[:4], []byte{byte(i >> 8), byte(i)})
		os.Stdout.Write(outbuf[:n])
		os.Stdout.Write([]byte(": "))

		for j := i; j < end; j++ {
			if j > i {
				if (j-i)%8 == 0 {
					os.Stdout.Write([]byte(" "))
				}
				if (j-i)%16 == 0 {
					os.Stdout.Write([]byte("  "))
				}
			}
			hex.Encode(outbuf[:2], data[j:j+1])
			os.Stdout.Write(outbuf[:2])
			os.Stdout.Write([]byte(" "))
		}
		os.Stdout.Write([]byte("\n"))
	}
}

func formatDescriptors(desc *DMADesc) {
	os.Stdout.Write([]byte("Descriptors:\n"))
	formatDescWord(0, &desc.Desc0)
	formatDescWord(1, &desc.Desc1)
	formatDescWord(2, &desc.Desc2)
	formatDescWord(3, &desc.Desc3)
}

func formatDescWord(i int, word *volatile.Register32) {
	os.Stdout.Write([]byte("  Word "))
	outbuf[0] = '0' + byte(i)
	os.Stdout.Write(outbuf[:1])
	os.Stdout.Write([]byte(": 0x"))

	// Write 8 hex digits
	b := word.Get()
	for shift := 28; shift >= 0; shift -= 4 {
		n := (b >> uint(shift)) & 0xF
		if n < 10 {
			outbuf[0] = '0' + byte(n)
		} else {
			outbuf[0] = 'a' + byte(n-10)
		}
		os.Stdout.Write(outbuf[:1])
	}
	os.Stdout.Write([]byte("\n"))
}
