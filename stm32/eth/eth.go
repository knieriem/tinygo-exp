package main

// This is the most minimal blinky example and should run almost everywhere.

import (
	"device/stm32"
	"errors"
	"machine"
	"runtime/volatile"
	"time"
	"unsafe"

	"github.com/knieriem/tinygo-exp/internal/phy/lan8742a"
	"github.com/knieriem/tinygo-exp/stm32/eth/internal/gmac"
)

func configureETHPins() {
	configureEthPin(ETH_REF_CLK)
	configureEthPin(ETH_RXD0)
	configureEthPin(ETH_RXD1)
	configureEthPin(ETH_TX_EN)
	configureEthPin(ETH_TXD0)
	configureEthPin(ETH_TXD1)
	configureEthPin(ETH_CRS_DV)
	configureEthPin(ETH_MDC)
	configureEthPin(ETH_MDIO)
}

func configureEthPin(pin machine.Pin) {
	pin.ConfigureAltFunc(machine.PinConfig{Mode: machine.PinModeETH}, 11)
}

const (
	TX_BUF_SIZE = 1524

	RDES3_OWN = 1 << 31
	// DMA RX Descriptor flags
	RDES3_BUF1V = 1 << 24
)

type DMADesc struct {
	Desc0 volatile.Register32 // Buffer1 address
	Desc1 volatile.Register32 // Not used
	Desc2 volatile.Register32 // Control: buffer1 length
	Desc3 volatile.Register32 // Status: OWN, LS, FS
}

//go:section .ram_nocache
var txDesc [4]gmac.DMADescTx

// TX buffer
//
//go:section .ram_nocache
var txBuf [TX_BUF_SIZE]uint8

//go:section .ram_nocache
var rxDesc [4]DMADesc

//go:section .ram_nocache
var rxBuf [TX_BUF_SIZE]uint8

// example Ethernet frame (broadcast)
var payload = []byte{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // dest MAC
	0x02, 0xd1, 0x91, 0x07, 0x02, 0x03, // src MAC
	0x88, 0xb5, // Ethertype: IPv4
	'H', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd', '!',
}

var rxDescBase = uint32(uintptr(unsafe.Pointer(&rxDesc[0])))

var txDescRing gmac.TXRing

func initMAC() {

	clear(txDesc[:])
	clear(rxDesc[:])
	clear(txBuf[:])

	MAC := stm32.Ethernet_MAC
	MTL := stm32.Ethernet_MTL
	DMA := stm32.Ethernet_DMA

	println("eth dma soft reset")
	gmac.DMASoftReset()

	mdio := gmac.SetupMDIO()
	mdio.SetBusyWait(func() error {
		time.Sleep(time.Millisecond)
		return nil
	})
	gmac.SetHardwareAddr([6]byte{0x02, 0xD1, 0x91, 0x07, 0x02, 0x03})

	phyDev.MDIO = mdio
	phyDev.Reset()

	// Enable TX/RX
	MAC.MACCR.SetBits(stm32.Ethernet_MACCR_RE | stm32.Ethernet_MACCR_TE |
		stm32.Ethernet_MACCR_FES | stm32.Ethernet_MACCR_CST)

	// Configure tx queue transmit operating mode
	MTL.MTLTxQOMR.ReplaceBits(stm32.Ethernet_MTL_MTLTxQOMR_TSF|
		0b10<<stm32.Ethernet_MTL_MTLTxQOMR_TXQEN_Pos|
		0b111<<stm32.Ethernet_MTL_MTLTxQOMR_TQS_Pos,
		stm32.Ethernet_MTL_MTLTxQOMR_TXQEN_Msk|
			stm32.Ethernet_MTL_MTLTxQOMR_TQS_Msk, 0)

	MTL.MTLRxQOMR.SetBits(stm32.Ethernet_MTL_MTLRxQOMR_RSF)

	txDescRing.SetDescriptors(txDesc[:])

	// Fill tx descriptor
	copy(txBuf[:], payload[:])
	println("eth tx setup dma")

	DMA.DMACRxCR.ReplaceBits(uint32(len(rxBuf))<<stm32.Ethernet_DMA_DMACRxCR_RBSZ_Pos, stm32.Ethernet_DMA_DMACRxCR_RBSZ_Msk, 0)

	DMA.DMACRxDLAR.Set(rxDescBase)
	DMA.DMACRxRLR.Set(1 - 1) // Only one descriptor (index 0 = ring length 1)
	DMA.DMACRxDTPR.Set(rxDescBase)

	d := &rxDesc[0]
	d.Desc0.Set(uint32(uintptr(unsafe.Pointer(&rxBuf[0]))))
	d.Desc1.Set(0)
	d.Desc2.Set(0)
	d.Desc3.Set(RDES3_OWN | RDES3_BUF1V)

	// Start TX and RX DMA engines
	DMA.DMACTxCR.SetBits(stm32.Ethernet_DMA_DMACTxCR_ST)
	DMA.DMACRxCR.SetBits(stm32.Ethernet_DMA_DMACRxCR_SR)

	waitForLinkUp()

	DMA.DMACRxDTPR.Set(rxDescBase + 16)

	println("DMA / RX/TX queue status:", DMA.DMACSR.Get(), MTL.MTLRxQDR.Get(), MTL.MTLTxQDR.Get())
}

func main() {
	pulseLED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	linkLED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rxLED.Configure(machine.PinConfig{Mode: machine.PinOutput})

	gmac.InitPeriph()
	configureETHPins()
	initMAC()

	println("init done")
	var txd *gmac.DMADescTx
	for {
		nTx, txCur := txDescRing.Update()
		if txd == nil || txd.IsAvail() {
			txBuf[len(payload)-1]++
			txd = txDescRing.Put(txBuf[:len(payload)])
		}
		println("DMA / RX/TX queue status:", rxDesc[0].Desc3.HasBits(RDES3_OWN), nTx, txCur, stm32.Ethernet_DMA.DMACSR.Get(), stm32.Ethernet_MTL.MTLRxQDR.Get(), stm32.Ethernet_MTL.MTLTxQDR.Get())
		rxd := &rxDesc[0]
		if !rxd.Desc3.HasBits(RDES3_OWN) {
			length := int((rxd.Desc2.Get() >> 16) & 0x3FFF)
			rxLED.Low()
			println("RX len", length)
			formatDescriptors(rxd)

			length = min(max(14, length), 256)
			dumpFrame(rxBuf[:length])

			rxd.Desc3.SetBits(RDES3_OWN | RDES3_BUF1V)
			stm32.Ethernet_DMA.DMACRxDTPR.Set(rxDescBase + 16)
			rxLED.High()
		}

		println("+")
		pulseLED.Low()
		time.Sleep(time.Millisecond * 500)

		println("-.")
		pulseLED.Low()
		time.Sleep(time.Millisecond * 500)

		machine.Watchdog.Update()
	}
}

var phyDev = lan8742a.PHY{
	Addr: 0,
}

func waitForLinkUp() error {
	// Wait for link up bit in BSR
	timeout := time.Now().Add(5 * time.Second)
	for time.Now().Before(timeout) {
		bsr, err := phyDev.LinkStatus()
		if err != nil {
			return err
		}
		if bsr.Up {
			println("link up", bsr.Speed)
			return nil
		}
		linkLED.Low()
		time.Sleep(time.Millisecond * 100)

		linkLED.High()
		time.Sleep(time.Millisecond * 100)
	}
	return errors.New("PHY link not up")
}
