//go:build nucleoh723zg

package main

import (
	"machine"
)

const (
	ETH_REF_CLK = machine.PA1
	ETH_RXD0    = machine.PC4
	ETH_RXD1    = machine.PC5

	ETH_TX_EN = machine.PG11
	ETH_TXD0  = machine.PG13
	ETH_TXD1  = machine.PB13

	ETH_CRS_DV = machine.PA7
	ETH_MDC    = machine.PC1
	ETH_MDIO   = machine.PA2

	pulseLED = machine.LED_GREEN
	linkLED  = machine.LED_YELLOW
	rxLED    = machine.LED_RED
)
