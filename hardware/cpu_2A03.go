// TODO: implement SBC
package hardware

import (
	"fmt"
	"os"
)

// Constants for stack start address and stack reset value
// The reason the NES stack ends at 253 bytes (0x01FD) rather than 256 bytes (0x01FF) is due to a hardware limitation.
// The top three addresses (0x01FD, 0x01FE, and 0x01FF) are reserved for the NES's interrupt vector table.
const STACK_START uint16 = 0x0100
const STACK_RESET uint8 = 0xfd

// Interrupt Vectors
const NMI uint16 = 0xFFFA // Non Maskable Interrupt
const RES uint16 = 0xFFFC // Reset Interrupt
const IRQ uint16 = 0xFFFE // Ordinary Interrupt Request

// general helpful functions
func extractBit(val uint8, pos uint8) uint8 {
	return (val & (1 << pos)) >> pos
}

type CPU struct {
	//registers
	accumulator     uint8
	index_x         uint8
	index_y         uint8
	status          uint8 // status flags
	program_counter uint16
	stack_pointer   uint8

	//memory
	memory []uint8
}

type Flags uint8

const (
	C Flags = iota //carry flag
	Z              //zero flag
	I              //interrupt disable
	D              //decimal mode
	B              //B flag
	X              //X - unused
	V              //overflow flag
	N              //negative flag
)

type AddressingMode int

const (
	modeImmediate AddressingMode = iota
	modeZeroPage
	modeAbsolute
	modeZeroPageX
	modeZeroPageY
	modeAbsoluteX
	modeAbsoluteY
	modeIndirectX
	modeIndirectY
	modeRelative
	modeAccumulator
	modeIndirect //only for JMP
	modeNoneAddressing
)

// helper functions to read and write memory
func (c *CPU) mem_read(address uint16) uint8 {
	return c.memory[address]
}
func (c *CPU) mem_write(address uint16, data uint8) {
	c.memory[address] = data
}

// 2A03 follows the little endian model to store 16 bit numbers
// least significant byte is stored before most significant byte
func (c *CPU) mem_read_16(address uint16) uint16 {
	lsb := uint16(c.mem_read(address))
	msb := uint16(c.mem_read(address + 1))
	return (msb << 8) | lsb
}

func (c *CPU) mem_write_16(address uint16, data uint16) {
	lsb := uint8(data & 0xFF)
	msb := uint8(data >> 8)
	c.mem_write(address, lsb)
	c.mem_write(address+1, msb)
}

// stack functions
func (c *CPU) push(data uint8) {
	c.memory[STACK_START+uint16(c.stack_pointer)] = data
	c.stack_pointer--
}

func (c *CPU) pop() uint8 {
	top := c.memory[STACK_START+uint16(c.stack_pointer)]
	c.stack_pointer++
	return top
}

func (c *CPU) push_16(data uint16) {
	lsb := uint8(data & 0xFF)
	msb := uint8(data >> 8)
	c.memory[STACK_START+uint16(c.stack_pointer)] = lsb
	c.stack_pointer--
	c.memory[STACK_START+uint16(c.stack_pointer)] = msb
	c.stack_pointer--
}

func (c *CPU) pop_16() uint16 {
	msb := uint16(c.memory[c.stack_pointer])
	c.stack_pointer++
	lsb := uint16(c.memory[c.stack_pointer])
	c.stack_pointer++
	return (msb << 8) | lsb
}

// Helper function to calculate the operand address based on addressing mode
func (c *CPU) address_operand(mode AddressingMode) uint16 {
	var address uint16
	switch mode {
	case modeImmediate:
		address = c.program_counter
	case modeZeroPage:
		address = uint16(c.mem_read(c.program_counter))
	case modeAbsolute:
		address = c.mem_read_16(c.program_counter)
	case modeZeroPageX:
		base_addr := c.mem_read(c.program_counter)
		address = uint16(base_addr + c.index_x)
	case modeZeroPageY:
		base_addr := c.mem_read(c.program_counter)
		address = uint16(base_addr + c.index_y)
	case modeAbsoluteX:
		base_addr := c.mem_read_16(c.program_counter)
		address = base_addr + uint16(c.index_x)
	case modeAbsoluteY:
		base_addr := c.mem_read_16(c.program_counter)
		address = base_addr + uint16(c.index_y)
	case modeIndirectX:
		base := c.mem_read(c.program_counter)
		var offset uint8 = base + c.index_x
		lsb := c.mem_read(uint16(offset))
		msb := c.mem_read(uint16(offset + 1))
		address = (uint16(msb) << 8) | uint16(lsb)
	case modeIndirectY:
		base := c.mem_read(c.program_counter)
		var offset uint8 = base + c.index_y
		lsb := c.mem_read(uint16(offset))
		msb := c.mem_read(uint16(offset + 1))
		address = (uint16(msb) << 8) | uint16(lsb)
	case modeRelative:
		address = c.program_counter
	case modeIndirect:
		lsb := uint16(c.mem_read(c.program_counter))
		msb := uint16(c.mem_read(c.program_counter + 1))
		indirectVector := (msb << 8) | lsb
		address_lsb := uint16(c.mem_read(indirectVector))
		address_msb := uint16(c.mem_read(indirectVector + 1))
		if indirectVector&0x00ff == 0x00ff {
			address_msb = address_lsb & 0xff00
		}

		address = (address_msb << 8) | address_lsb
	}

	return address
}

// helper functions for flags
func (c *CPU) setFlags(flags ...Flags) {
	for _, f := range flags {
		c.status |= (1 << f)
	}
}

func (c *CPU) resetFlags(flags ...Flags) {
	for _, f := range flags {
		c.status &= ^(1 << f)
	}
}

func (c *CPU) setFlagValue(flag Flags, val uint8) {
	if val == 1 {
		c.setFlags(flag)
	} else {
		c.resetFlags(flag)
	}

}

func (c *CPU) getFlagValue(flag Flags) uint8 {
	return (c.status & (1 << flag)) >> (flag)
}

func (c *CPU) updateZandN(val uint8) {
	//check for zero
	if val == 0 {
		c.setFlags(Z)
	} else {
		c.resetFlags(Z)
	}

	//check for negative
	if val&128 == 128 {
		c.setFlags(Z)
	} else {
		c.resetFlags(Z)
	}
}

// INSTRUCTIONS
func (c *CPU) adc(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	res := c.accumulator + value + c.getFlagValue(C)
	if res > 255 {
		c.setFlags(C)
	}
	if res > 127 {
		c.setFlags(V)
	}

	c.accumulator = res
	c.updateZandN(c.accumulator)

}

func (c *CPU) and(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	c.accumulator &= value
	c.updateZandN(c.accumulator)
}

func (c *CPU) asl(mode AddressingMode) {
	if mode == modeAccumulator {
		c.setFlagValue(C, extractBit(c.accumulator, 7))
		c.accumulator = c.accumulator << 1
	} else {
		address := c.address_operand(mode)
		value := c.mem_read(address)
		c.setFlagValue(C, extractBit(value, 7))
		value = value << 1
		c.mem_write(address, value)
	}

	c.updateZandN(c.status)
}

func (c *CPU) bcc() {
	if c.getFlagValue(C) == 0 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bcs() {
	if c.getFlagValue(C) == 1 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) beq() {
	if c.getFlagValue(Z) == 1 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bit(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	res := c.accumulator & value
	if res == 0 {
		c.setFlags(Z)
	} else {
		c.resetFlags(Z)
	}
	c.setFlagValue(V, extractBit(res, 6))
	c.setFlagValue(N, extractBit(res, 7))
}

func (c *CPU) bmi() {
	if c.getFlagValue(N) == 1 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bne() {
	if c.getFlagValue(C) == 0 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bpl() {
	if c.getFlagValue(N) == 0 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) brk() {
	c.push_16(c.program_counter)
	c.push(c.status)
	c.program_counter = c.mem_read_16(IRQ)
	c.setFlags(B)
}

func (c *CPU) bvc() {
	if c.getFlagValue(V) == 0 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bvs() {
	if c.getFlagValue(V) == 1 {
		address := c.address_operand(modeRelative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) clc() {
	c.resetFlags(C)
}

func (c *CPU) cld() {
	c.resetFlags(D)
}

func (c *CPU) cli() {
	c.resetFlags(I)
}

func (c *CPU) clv() {
	c.resetFlags(V)
}

func (c *CPU) cmp(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	res := c.accumulator - value
	if res >= uint8(0) {
		c.setFlags(C)
	}
	c.updateZandN(res)
}

func (c *CPU) cpx(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	res := c.index_x - value
	if res >= uint8(0) {
		c.setFlags(C)
	}
	c.updateZandN(res)

}

func (c *CPU) cpy(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	res := c.index_y - value
	if res >= uint8(0) {
		c.setFlags(C)
	}
	c.updateZandN(res)

}

func (c *CPU) dec(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	value--
	c.mem_write(address, value)
	c.updateZandN(value)
}

func (c *CPU) dex() {
	c.index_x--
	c.updateZandN(c.index_x)
}

func (c *CPU) dey() {
	c.index_y--
	c.updateZandN(c.index_y)
}

func (c *CPU) eor(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	c.accumulator = c.accumulator ^ value
	c.updateZandN(c.accumulator)
}

func (c *CPU) inc(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	value++
	c.mem_write(address, value)
	c.updateZandN(value)
}

func (c *CPU) inx() {
	c.index_x++
	c.updateZandN(c.index_x)
}

func (c *CPU) iny() {
	c.index_y++
	c.updateZandN(c.index_y)
}

func (c *CPU) jmp(mode AddressingMode) {
	address := c.address_operand(mode)
	c.program_counter = address

}

func (c *CPU) jsr() {
	c.push_16(c.program_counter - 1)
	address := c.address_operand(modeAbsolute)
	c.program_counter = address
}

func (c *CPU) lda(mode AddressingMode) {
	address := c.address_operand(mode)
	c.accumulator = c.mem_read(address)
	c.updateZandN(c.accumulator)
}

func (c *CPU) ldx(mode AddressingMode) {
	address := c.address_operand(mode)
	c.index_x = c.mem_read(address)
	c.updateZandN(c.index_x)
}

func (c *CPU) ldy(mode AddressingMode) {
	address := c.address_operand(mode)
	c.index_y = c.mem_read(address)
	c.updateZandN(c.index_y)
}

func (c *CPU) lsr(mode AddressingMode) {
	if mode == modeAccumulator {
		c.setFlagValue(C, extractBit(c.accumulator, 0))
		c.accumulator = c.accumulator >> 1
		c.updateZandN(c.accumulator)
	} else {
		address := c.address_operand(mode)
		value := c.mem_read(address)
		c.setFlagValue(C, extractBit(value, 0))
		value = value >> 1
		c.mem_write(address, value)
		c.updateZandN(value)
	}
}

func (c *CPU) nop() {

}

func (c *CPU) ora(mode AddressingMode) {
	address := c.address_operand(mode)
	value := c.mem_read(address)
	c.accumulator = c.accumulator | value
	c.updateZandN(c.accumulator)
}

func (c *CPU) pha() {
	c.push(c.accumulator)
}

func (c *CPU) php() {
	c.push(c.status)
}

func (c *CPU) pla() {
	c.accumulator = c.pop()
	c.updateZandN(c.accumulator)
}

func (c *CPU) plp() {
	c.status = c.pop()
	c.updateZandN(c.status)
}

func (c *CPU) rol(mode AddressingMode) {
	if mode == modeAccumulator {
		prevCarry := extractBit(c.status, 0)
		c.setFlagValue(C, extractBit(c.accumulator, 7))
		c.accumulator = (c.accumulator << 1) | prevCarry
		c.updateZandN(c.accumulator)
	} else {
		address := c.address_operand(mode)
		value := c.mem_read(address)
		prevCarry := extractBit(c.status, 0)
		c.setFlagValue(C, extractBit(value, 7))
		value = (value << 1) | prevCarry
		c.mem_write(address, value)
		c.updateZandN(value)
	}
}

func (c *CPU) ror(mode AddressingMode) {
	if mode == modeAccumulator {
		prevCarry := extractBit(c.status, 0)
		c.setFlagValue(C, extractBit(c.accumulator, 0))
		c.accumulator = (c.accumulator >> 1) | (prevCarry << 7)
		c.updateZandN(c.accumulator)
	} else {
		address := c.address_operand(mode)
		value := c.mem_read(address)
		prevCarry := extractBit(c.status, 0)
		c.setFlagValue(C, extractBit(c.accumulator, 0))
		value = (value >> 1) | (prevCarry << 7)
		c.mem_write(address, value)
		c.updateZandN(value)
	}
}

func (c *CPU) rti() {
	c.status = c.pop()
	c.program_counter = c.pop_16()
}

func (c *CPU) rts() {
	c.program_counter = c.pop_16()
}

func (c *CPU) sec() {
	c.setFlagValue(C, 1)
}

func (c *CPU) sed() {
	c.setFlagValue(D, 1)
}

func (c *CPU) sei() {
	c.setFlagValue(I, 1)
}

func (c *CPU) sta(mode AddressingMode) {
	address := c.address_operand(mode)
	c.mem_write(address, c.accumulator)
}

func (c *CPU) stx(mode AddressingMode) {
	address := c.address_operand(mode)
	c.mem_write(address, c.index_x)
}

func (c *CPU) sty(mode AddressingMode) {
	address := c.address_operand(mode)
	c.mem_write(address, c.index_y)
}

func (c *CPU) tax() {
	c.index_x = c.accumulator
	c.updateZandN(c.index_x)
}

func (c *CPU) tay() {
	c.index_y = c.accumulator
	c.updateZandN(c.index_y)
}

func (c *CPU) tsx() {
	c.index_x = c.status
	c.updateZandN(c.index_x)
}

func (c *CPU) txa() {
	c.accumulator = c.index_x
	c.updateZandN(c.accumulator)
}

func (c *CPU) txs() {
	c.status = c.index_x
	c.updateZandN(c.index_x)
}

func (c *CPU) tya() {
	c.accumulator = c.index_y
	c.updateZandN(c.accumulator)
}

func (c *CPU) load(instructions []uint8) {
	for i, val := range instructions {
		c.memory[0x8000+i] = val
	}
	c.mem_write_16(0xFFFC, 0x8000) // set the reset vector https://en.wikipedia.org/wiki/Reset_vector
}

func (c *CPU) reset() {
	c.accumulator = 0
	c.index_x = 0
	c.index_y = 0
	c.status = 0b00100100
	c.stack_pointer = STACK_RESET
	c.program_counter = c.mem_read_16(0xFFFC)

}

func (c *CPU) Interpret() {
	for {
		opcode := c.memory[c.program_counter]
		c.program_counter++
		switch opcode {

		case 0x69:
			c.adc(modeImmediate)
			c.program_counter++
		case 0x65:
			c.adc(modeZeroPage)
			c.program_counter++
		case 0x75:
			c.adc(modeZeroPageX)
			c.program_counter++
		case 0x6D:
			c.adc(modeAbsolute)
			c.program_counter += 2
		case 0x7d:
			c.adc(modeAbsoluteX)
			c.program_counter += 2
		case 0x79:
			c.adc(modeAbsoluteY)
			c.program_counter += 2
		case 0x61:
			c.adc(modeIndirectX)
			c.program_counter++
		case 0x71:
			c.adc(modeIndirectY)
			c.program_counter++

		case 0x29:
			c.and(modeImmediate)
			c.program_counter++
		case 0x25:
			c.and(modeZeroPage)
			c.program_counter++
		case 0x35:
			c.and(modeZeroPageX)
			c.program_counter++
		case 0x2d:
			c.and(modeAbsolute)
			c.program_counter += 2
		case 0x3d:
			c.and(modeAbsoluteX)
			c.program_counter += 2
		case 0x39:
			c.and(modeAbsoluteY)
			c.program_counter += 2
		case 0x21:
			c.and(modeIndirectX)
			c.program_counter++
		case 0x31:
			c.and(modeIndirectY)
			c.program_counter++

		case 0x0a:
			c.asl(modeAccumulator)
		case 0x06:
			c.asl(modeZeroPage)
			c.program_counter++
		case 0x16:
			c.asl(modeZeroPageX)
			c.program_counter++
		case 0x0e:
			c.asl(modeAbsolute)
			c.program_counter += 2
		case 0x1e:
			c.asl(modeAbsoluteX)
			c.program_counter += 2

		case 0x90:
			c.bcc()
			c.program_counter++

		case 0xb0:
			c.bcs()
			c.program_counter++

		case 0xf0:
			c.beq()
			c.program_counter++

		case 0x24:
			c.bit(modeZeroPage)
			c.program_counter++
		case 0x2c:
			c.bit(modeAbsolute)
			c.program_counter += 2

		case 0x30:
			c.bmi()
			c.program_counter++

		case 0xd0:
			c.bne()
			c.program_counter++

		case 0x10:
			c.bpl()
			c.program_counter++

		case 0x00:
			c.brk()

		case 0x50:
			c.bvc()
			c.program_counter++

		case 0x70:
			c.bvs()
			c.program_counter++

		case 0x18:
			c.clc()
		case 0xd8:
			c.cld()
		case 0x58:
			c.cli()
		case 0xb8:
			c.clv()

		case 0xc9:
			c.cmp(modeImmediate)
			c.program_counter++
		case 0xc5:
			c.cmp(modeZeroPage)
			c.program_counter++
		case 0xd5:
			c.cmp(modeZeroPageX)
			c.program_counter++
		case 0xcd:
			c.cmp(modeAbsolute)
			c.program_counter += 2
		case 0xdd:
			c.cmp(modeAbsoluteX)
			c.program_counter += 2
		case 0xd9:
			c.cmp(modeAbsoluteY)
			c.program_counter += 2
		case 0xc1:
			c.cmp(modeIndirectX)
			c.program_counter++
		case 0xd1:
			c.cmp(modeIndirectY)
			c.program_counter++

		case 0xe0:
			c.cpx(modeImmediate)
			c.program_counter++
		case 0xe4:
			c.cpx(modeZeroPage)
			c.program_counter++
		case 0xec:
			c.cpx(modeAbsolute)
			c.program_counter += 2

		case 0xc0:
			c.cpy(modeImmediate)
			c.program_counter++

		case 0xc4:
			c.cpy(modeZeroPage)
			c.program_counter++
		case 0xcc:
			c.cpy(modeAbsolute)
			c.program_counter += 2

		case 0xc6:
			c.dec(modeZeroPage)
			c.program_counter++
		case 0xd6:
			c.dec(modeZeroPageX)
			c.program_counter++
		case 0xce:
			c.dec(modeAbsolute)
			c.program_counter += 2
		case 0xde:
			c.dec(modeAbsoluteX)
			c.program_counter += 2

		case 0xca:
			c.dex()
		case 0x88:
			c.dey()

		case 0x49:
			c.eor(modeImmediate)
			c.program_counter++
		case 0x45:
			c.eor(modeZeroPage)
			c.program_counter++
		case 0x55:
			c.eor(modeZeroPageX)
			c.program_counter++
		case 0x4d:
			c.eor(modeAbsolute)
			c.program_counter += 2
		case 0x5d:
			c.eor(modeAbsoluteX)
			c.program_counter += 2
		case 0x59:
			c.eor(modeAbsoluteY)
			c.program_counter += 2
		case 0x41:
			c.eor(modeIndirectX)
			c.program_counter++
		case 0x51:
			c.eor(modeIndirectY)
			c.program_counter++

		case 0xe6:
			c.inc(modeZeroPage)
			c.program_counter++
		case 0xf6:
			c.inc(modeZeroPageX)
			c.program_counter++
		case 0xee:
			c.inc(modeAbsolute)
			c.program_counter += 2
		case 0xfe:
			c.inc(modeAbsoluteX)
			c.program_counter += 2

		case 0xe8:
			c.inx()
		case 0xc8:
			c.iny()

		case 0x4c:
			c.jmp(modeAbsolute)
			c.program_counter += 2
		case 0x6c:
			c.jmp(modeIndirect)
			c.program_counter += 2

		case 0x20:
			c.jsr()
			c.program_counter += 2

		case 0xa9:
			c.lda(modeImmediate)
			c.program_counter++
		case 0xa5:
			c.lda(modeZeroPage)
			c.program_counter++
		case 0xb5:
			c.lda(modeZeroPageX)
			c.program_counter++
		case 0xad:
			c.lda(modeAbsolute)
			c.program_counter += 2
		case 0xbd:
			c.lda(modeAbsoluteX)
			c.program_counter += 2
		case 0xb9:
			c.lda(modeAbsoluteY)
			c.program_counter += 2
		case 0xa1:
			c.lda(modeIndirectX)
			c.program_counter++
		case 0xb1:
			c.lda(modeIndirectY)
			c.program_counter++

		case 0xa2:
			c.ldx(modeImmediate)
			c.program_counter++
		case 0xa6:
			c.ldx(modeZeroPage)
			c.program_counter++
		case 0xae:
			c.ldx(modeAbsolute)
			c.program_counter += 2
		case 0xbe:
			c.ldx(modeAbsoluteY)
			c.program_counter += 2

		case 0xa0:
			c.ldy(modeImmediate)
			c.program_counter++
		case 0xa4:
			c.ldy(modeZeroPage)
			c.program_counter++
		case 0xb4:
			c.ldy(modeZeroPageX)
			c.program_counter++
		case 0xac:
			c.ldy(modeAbsolute)
			c.program_counter += 2
		case 0xbc:
			c.ldy(modeAbsoluteX)
			c.program_counter += 2

		case 0x4a:
			c.lsr(modeAccumulator)
		case 0x46:
			c.lsr(modeZeroPage)
			c.program_counter++
		case 0x56:
			c.lsr(modeZeroPageX)
			c.program_counter++
		case 0x4e:
			c.lsr(modeAbsolute)
			c.program_counter += 2
		case 0x5e:
			c.lsr(modeAbsoluteX)
			c.program_counter += 2

		case 0xea:
			c.nop()

		case 0x09:
			c.ora(modeImmediate)
			c.program_counter++
		case 0x05:
			c.ora(modeZeroPage)
			c.program_counter++
		case 0x015:
			c.ora(modeZeroPageX)
			c.program_counter++
		case 0x0d:
			c.ora(modeAbsolute)
			c.program_counter += 2
		case 0x1d:
			c.ora(modeAbsoluteX)
			c.program_counter += 2
		case 0x19:
			c.ora(modeAbsoluteY)
			c.program_counter += 2
		case 0x01:
			c.ora(modeIndirectX)
			c.program_counter++
		case 0x11:
			c.ora(modeIndirectY)
			c.program_counter++

		case 0x48:
			c.pha()
		case 0x08:
			c.php()
		case 0x68:
			c.pla()
		case 0x28:
			c.plp()

		case 0x2a:
			c.rol(modeAccumulator)
		case 0x26:
			c.rol(modeZeroPage)
			c.program_counter++
		case 0x36:
			c.rol(modeZeroPageX)
			c.program_counter++
		case 0x2e:
			c.rol(modeAbsolute)
			c.program_counter += 2
		case 0x3e:
			c.rol(modeAbsoluteX)
			c.program_counter += 2

		case 0x6a:
			c.ror(modeAccumulator)
		case 0x66:
			c.ror(modeZeroPage)
			c.program_counter++
		case 0x76:
			c.ror(modeZeroPageX)
			c.program_counter++
		case 0x6e:
			c.ror(modeAbsolute)
			c.program_counter += 2
		case 0x7e:
			c.ror(modeAbsoluteX)
			c.program_counter += 2

		case 0x40:
			c.rti()
		case 0x060:
			c.rts()

			//TODO: Implement SBC
		case 0x38:
			c.sec()
		case 0xf8:
			c.sed()
		case 0x78:
			c.sei()

		case 0x85:
			c.sta(modeZeroPage)
			c.program_counter++
		case 0x95:
			c.sta(modeZeroPageX)
			c.program_counter++
		case 0x8d:
			c.sta(modeAbsolute)
			c.program_counter += 2
		case 0x9d:
			c.sta(modeAbsoluteX)
			c.program_counter += 2
		case 0x99:
			c.sta(modeAbsoluteY)
			c.program_counter += 2
		case 0x81:
			c.sta(modeIndirectX)
			c.program_counter++
		case 0x91:
			c.sta(modeIndirectY)
			c.program_counter++

		case 0x86:
			c.stx(modeZeroPage)
			c.program_counter++
		case 0x96:
			c.stx(modeZeroPageY)
			c.program_counter++
		case 0x8e:
			c.stx(modeAbsolute)
			c.program_counter += 2

		case 0x84:
			c.sty(modeZeroPage)
			c.program_counter++
		case 0x94:
			c.sty(modeZeroPageX)
			c.program_counter++
		case 0x8c:
			c.sty(modeAbsolute)
			c.program_counter++

		case 0xaa:
			c.tax()
		case 0xa8:
			c.tay()
		case 0xba:
			c.tsx()
		case 0x8a:
			c.txa()
		case 0x9a:
			c.txs()
		case 0x98:
			c.tya()

		default:
			fmt.Fprintf(os.Stdout, "UNDEFINED BEHAVIOUR %v at %v", opcode, c.program_counter)

		}
	}
}

func (c *CPU) Load_and_interpret(instructions []uint8) {
	c.load(instructions)
	c.reset()
	c.Interpret()
}

func NewCPU() CPU {
	return CPU{
		accumulator:     0,
		index_x:         0,
		index_y:         0,
		status:          0b00100100,
		program_counter: 0,
		stack_pointer:   STACK_RESET,
		memory:          make([]uint8, 0x10000),
		//memory takes default 0 with size defined
	}
}
