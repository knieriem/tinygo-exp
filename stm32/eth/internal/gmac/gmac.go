package gmac

import (
	"device/stm32"
	"os"
	"runtime/volatile"
	"unsafe"
)

var mac = stm32.Ethernet_MAC
var dma = stm32.Ethernet_DMA

func DMASoftReset() error {
	dma.DMAMR.SetBits(stm32.Ethernet_DMA_DMAMR_SWR)

	for dma.DMAMR.HasBits(stm32.Ethernet_DMA_DMAMR_SWR) {
	}
	return nil
}

func SetHardwareAddr(addr [6]byte) {
	mac.MACA0HR.Set(uint32(addr[5])<<8 | uint32(addr[4]))
	mac.MACA0LR.Set(uint32(addr[3])<<24 | uint32(addr[2])<<16 | uint32(addr[1])<<8 | uint32(addr[0]))
}

// A simple interface to the DWMAC1000 / GMAC derived Ethernet device
// used in STM32H7

const (
	tdes2B1LMask = (1 << 14) - 1
	tdes2B1LPos  = 0

	tdes3OWN = 1 << 31
	tdes3FD  = 1 << 28
	tdes3LD  = 1 << 29

	// DMA RX Descriptor flags
	rdes3BUF1V = 1 << 24
)

type DMADescTx struct {
	tdes0 volatile.Register32
	tdes1 volatile.Register32
	tdes2 volatile.Register32
	tdes3 volatile.Register32
}

func (d *DMADescTx) setData(data []byte) {
	d.tdes0.Set(uint32(uintptr(unsafe.Pointer(&data[0]))))
	d.tdes2.Set((uint32(len(data)) << tdes2B1LPos) & tdes2B1LMask)
	d.tdes3.SetBits(tdes3FD | tdes3LD)
}

func (d *DMADescTx) setOwnerDMA() {
	d.tdes3.SetBits(tdes3OWN)
}
func (d *DMADescTx) IsAvail() bool {
	return !d.tdes3.HasBits(tdes3OWN)
}

type TXRing struct {
	base uint32
	buf  []DMADescTx
	mask int
	cur  int
	tail int
}

func (ring *TXRing) SetDescriptors(buf []DMADescTx) {
	ring.cur = 0
	ring.tail = 0
	ring.base = uint32(uintptr(unsafe.Pointer(&buf[0])))

	n := 1
	for {
		next := n << 1
		if next > len(buf) {
			break
		}
		n = next
	}

	ring.buf = buf[:n]
	ring.mask = n - 1

	println("setdesc", ring.base, n, ring.mask)
	dma.DMACTxDLAR.Set(ring.base)
	dma.DMACTxRLR.Set(uint32(n) - 1)
	dma.DMACTxDTPR.Set(ring.base)
}

func (ring *TXRing) Put(b []byte) *DMADescTx {
	w := ring.tail & ring.mask
	d := &ring.buf[w]
	if !d.IsAvail() {
		return nil
	}
	if n := len(ring.buf); n > 1 {
		if ring.tail-ring.cur+1 == len(ring.buf) {
			return nil
		}
	}
	d.setData(b)
	d.setOwnerDMA()
	d.dump()
	dma.DMACTxDTPR.Set(ring.base + uint32(w+1)*16) // trigger DMA
	ring.tail++
	return d
}

func (ring *TXRing) Update() (pending, cur int) {
	for ring.cur < ring.tail {
		cur := ring.cur & ring.mask
		d := &ring.buf[cur]
		if !d.IsAvail() {
			break
		}
		ring.cur++
	}
	return ring.tail - ring.cur, cur
}

func (d *DMADescTx) dump() {
	formatDescWord(0, &d.tdes0)
	formatDescWord(1, &d.tdes1)
	formatDescWord(2, &d.tdes2)
	formatDescWord(3, &d.tdes3)
}

func formatDescWord(i int, word *volatile.Register32) {
	var outbuf [9]byte

	os.Stdout.Write([]byte("  Word "))
	outbuf[0] = '0' + byte(i)
	os.Stdout.Write(outbuf[:1])
	os.Stdout.Write([]byte(": 0x"))

	// Write 8 hex digits
	b := word.Get()
	for i := range 8 {
		n := b & 0xF
		if n < 10 {
			outbuf[7-i] = '0' + byte(n)
		} else {
			outbuf[7-i] = 'a' + byte(n-10)
		}
		b >>= 4
	}
	outbuf[8] = '\n'
	os.Stdout.Write(outbuf[:9])
}

func InitPeriph() {
	stm32.RCC.APB4ENR.SetBits(stm32.RCC_APB4ENR_SYSCFGEN)
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_ETH1MACEN | stm32.RCC_AHB1ENR_ETH1TXEN | stm32.RCC_AHB1ENR_ETH1RXEN)
	_ = stm32.RCC.APB4ENR.Get()

	stm32.RCC.AHB2ENR.SetBits(stm32.RCC_AHB2ENR_SRAM1EN)

	stm32.SYSCFG.PMCR.ReplaceBits(0b100<<stm32.SYSCFG_PMCR_EPIS_Pos, stm32.SYSCFG_PMCR_EPIS_Msk, 0)
	_ = stm32.SYSCFG.PMCR.Get()
}

// Enable should be called after the link is up.
func Enable() {
	mac.MACCR.SetBits(stm32.Ethernet_MACCR_RE | stm32.Ethernet_MACCR_TE)
}
