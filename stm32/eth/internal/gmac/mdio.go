package gmac

import "device/stm32"

func SetupMDIO() *MDIO {
	mac.MACMDIOAR.ReplaceBits(0b010<<stm32.Ethernet_MACMDIOAR_CR_Pos, stm32.Ethernet_MACMDIOAR_CR_Msk, 0)
	return &devMDIO
}

var devMDIO MDIO

type MDIO struct {
	busyWait BusyWaitFunc
}

func (md *MDIO) SetBusyWait(wait func() error) {
	md.busyWait = wait
}

type BusyWaitFunc func() error

func (md *MDIO) ReadReg(phyAddr, regAddr uint8) (uint16, error) {

	// Set up the MDIO address register for a read
	// (bits depend on STM32 reference manual)
	mac.MACMDIOAR.ReplaceBits(uint32(phyAddr)<<stm32.Ethernet_MACMDIOAR_PA_Pos|
		uint32(regAddr)<<stm32.Ethernet_MACMDIOAR_RDA_Pos|
		0b11<<stm32.Ethernet_MACMDIOAR_GOC_Pos|
		stm32.Ethernet_MACMDIOAR_MB,
		stm32.Ethernet_MACMDIOAR_PA_Msk|
			stm32.Ethernet_MACMDIOAR_RDA_Msk|
			stm32.Ethernet_MACMDIOAR_GOC_Msk|
			stm32.Ethernet_MACMDIOAR_MB_Msk, 0)

	// Wait for the busy bit to clear
	for mac.MACMDIOAR.HasBits(stm32.Ethernet_MACMDIOAR_MB) {
		if md.busyWait != nil {
			if err := md.busyWait(); err != nil {
				return 0, err
			}
		}
	}

	// Read the data register
	data := uint16(mac.MACMDIODR.Get() & stm32.Ethernet_MACMDIODR_MD_Msk)
	println("read mdio", phyAddr, regAddr, data)
	return data, nil
}

func (md *MDIO) WriteReg(phyAddr, regAddr uint8, data uint16) error {

	println("write mdio", phyAddr, regAddr, data)
	// Write data
	mac.MACMDIODR.Set(uint32(data))

	mac.MACMDIOAR.ReplaceBits(uint32(phyAddr)<<stm32.Ethernet_MACMDIOAR_PA_Pos|
		uint32(regAddr)<<stm32.Ethernet_MACMDIOAR_RDA_Pos|
		0b01<<stm32.Ethernet_MACMDIOAR_GOC_Pos|
		stm32.Ethernet_MACMDIOAR_MB,
		stm32.Ethernet_MACMDIOAR_PA_Msk|
			stm32.Ethernet_MACMDIOAR_RDA_Msk|
			stm32.Ethernet_MACMDIOAR_GOC_Msk|
			stm32.Ethernet_MACMDIOAR_MB_Msk, 0)

	// Wait for busy bit to clear
	for mac.MACMDIOAR.HasBits(stm32.Ethernet_MACMDIOAR_MB) {
		if md.busyWait != nil {
			if err := md.busyWait(); err != nil {
				return err
			}
		}
	}
	return nil
}
