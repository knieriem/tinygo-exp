package lan8742a

import (
	"errors"
)

const (
	regBCR = 0  // Basic Control Register
	regBSR = 1  // Basic Status Register
	regSSR = 31 // Special Control/Status Register

	bcrRESET       = 1 << 15
	bsrLINK_STATUS = 1 << 2

	ssrAUTODONE     = 1 << 12
	ssrHCDSPEEDPos  = 2
	ssrHDCSPEEDMask = 0b111 << 2
)

type MDIO interface {
	ReadReg(phyAddr, regAddr uint8) (uint16, error)
	WriteReg(phyAddr, regAddr uint8, value uint16) error
}

type PHY struct {
	Addr     uint8
	MDIO     MDIO
	busyWait func()
}

func (phy *PHY) Detect() error {
	for i := 0; i < 63; i++ {
		id, err := phy.MDIO.ReadReg(uint8(i), 2)
		if err != nil {
			continue
		}
		if id == 0xFFFF {
			continue
		}
		phy.Addr = uint8(i)
		return nil
	}
	return ErrNotFound
}

var ErrNotFound = errors.New("phy not found")

func (phy *PHY) Reset() error {
	phy.MDIO.WriteReg(phy.Addr, regBCR, bcrRESET)

	for {
		bcr, err := phy.MDIO.ReadReg(phy.Addr, regBCR)
		if err != nil {
			continue
		}
		if bcr&bcrRESET == 0 {
			break
		}
		if phy.busyWait != nil {
			phy.busyWait()
		}
	}
	return nil
}

type LinkStatus struct {
	Up             bool
	AutoNegotiated bool
	Speed          uint16
	FullDuplex     bool
}

func (phy *PHY) LinkStatus() (LinkStatus, error) {
	var lst LinkStatus
	bsr, err := phy.MDIO.ReadReg(phy.Addr, regBSR)
	if err != nil {
		return lst, err
	}
	if bsr&bsrLINK_STATUS == 0 {
		return lst, nil
	}
	lst.Up = true

	ssr, err := phy.MDIO.ReadReg(phy.Addr, regBSR)
	if err != nil {
		return lst, err
	}

	if ssr&ssrAUTODONE != 0 {
		lst.AutoNegotiated = true
	} else {
		return lst, err
	}

	fullDuplex := true
	switch (ssr & ssrHDCSPEEDMask) >> ssrHCDSPEEDPos {
	case 0b001:
		fullDuplex = false
		fallthrough
	case 0b101:
		lst.Speed = 10
	case 0b010:
		fullDuplex = false
		fallthrough
	case 0b110:
		lst.Speed = 100
	default:
		lst.AutoNegotiated = false
		return lst, nil
	}
	lst.FullDuplex = fullDuplex
	return lst, nil
}
