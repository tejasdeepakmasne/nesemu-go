package hardware

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
	Immediate AddressingMode = iota
	ZeroPage
	Absolute
	ZeroPageX
	ZeroPageY
	AbsoluteX
	AbsoluteY
	IndirectX
	IndirectY
	Relative
	Accumulator
	NoneAddressing
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
	c.memory[c.stack_pointer] = data
	c.stack_pointer--
}

func (c *CPU) pop() uint8 {
	top := c.memory[c.stack_pointer]
	c.stack_pointer++
	return top
}

func (c *CPU) push_16(data uint16) {
	lsb := uint8(data & 0xFF)
	msb := uint8(data >> 8)
	c.memory[c.stack_pointer] = lsb
	c.stack_pointer--
	c.memory[c.stack_pointer] = msb
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
	case Immediate:
		address = c.program_counter
	case ZeroPage:
		address = uint16(c.mem_read(c.program_counter))
	case Absolute:
		address = c.mem_read_16(c.program_counter)
	case ZeroPageX:
		base_addr := c.mem_read(c.program_counter)
		address = uint16(base_addr + c.index_x)
	case ZeroPageY:
		base_addr := c.mem_read(c.program_counter)
		address = uint16(base_addr + c.index_y)
	case AbsoluteX:
		base_addr := c.mem_read_16(c.program_counter)
		address = base_addr + uint16(c.index_x)
	case AbsoluteY:
		base_addr := c.mem_read_16(c.program_counter)
		address = base_addr + uint16(c.index_y)
	case IndirectX:
		base := c.mem_read(c.program_counter)
		var offset uint8 = base + c.index_x
		lsb := c.mem_read(uint16(offset))
		msb := c.mem_read(uint16(offset + 1))
		address = (uint16(msb) << 8) | uint16(lsb)
	case IndirectY:
		base := c.mem_read(c.program_counter)
		var offset uint8 = base + c.index_y
		lsb := c.mem_read(uint16(offset))
		msb := c.mem_read(uint16(offset + 1))
		address = (uint16(msb) << 8) | uint16(lsb)
	case Relative:
		address = c.program_counter
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
	if mode == Accumulator {
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
		address := c.address_operand(Relative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bcs() {
	if c.getFlagValue(C) == 1 {
		address := c.address_operand(Relative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) beq() {
	if c.getFlagValue(Z) == 1 {
		address := c.address_operand(Relative)
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
		address := c.address_operand(Relative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bne() {
	if c.getFlagValue(C) == 0 {
		address := c.address_operand(Relative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bpl() {
	if c.getFlagValue(N) == 0 {
		address := c.address_operand(Relative)
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
		address := c.address_operand(Relative)
		value := c.mem_read(address)
		c.program_counter += uint16(value)
	}
}

func (c *CPU) bvs() {
	if c.getFlagValue(V) == 1 {
		address := c.address_operand(Relative)
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

func (c *CPU) load(instructions []uint8) {
	for i, val := range instructions {
		c.memory[8000+i] = val
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
			c.adc(Immediate)
			c.program_counter++
		case 0x65:
			c.adc(ZeroPage)
			c.program_counter++
		case 0x75:
			c.adc(ZeroPageX)
			c.program_counter++
		case 0x6D:
			c.adc(Absolute)
			c.program_counter += 2
		case 0x7d:
			c.adc(AbsoluteX)
			c.program_counter += 2
		case 0x79:
			c.adc(AbsoluteY)
			c.program_counter += 2
		case 0x61:
			c.adc(IndirectX)
			c.program_counter++
		case 0x71:
			c.adc(IndirectY)
			c.program_counter++

		case 0x29:
			c.adc(Immediate)
			c.program_counter++
		case 0x25:
			c.adc(ZeroPage)
			c.program_counter++
		case 0x35:
			c.adc(ZeroPageX)
			c.program_counter++
		case 0x2d:
			c.adc(Absolute)
			c.program_counter += 2
		case 0x3d:
			c.adc(AbsoluteX)
			c.program_counter += 2
		case 0x39:
			c.adc(AbsoluteY)
			c.program_counter += 2
		case 0x21:
			c.adc(IndirectX)
			c.program_counter++
		case 0x31:
			c.adc(IndirectY)
			c.program_counter++

		case 0x0a:
			c.asl(Accumulator)
		case 0x06:
			c.asl(ZeroPage)
			c.program_counter++
		case 0x16:
			c.asl(ZeroPageX)
			c.program_counter++
		case 0x0e:
			c.asl(Absolute)
			c.program_counter += 2
		case 0x1e:
			c.asl(AbsoluteX)
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
			c.bit(ZeroPage)
			c.program_counter++
		case 0x2c:
			c.bit(Absolute)
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
			c.clv()

		case 0xc9:
			c.cmp(Immediate)
			c.program_counter++
		case 0xc5:
			c.cmp(ZeroPage)
			c.program_counter++
		case 0xd5:
			c.cmp(ZeroPageX)
			c.program_counter++
		case 0xcd:
			c.cmp(Absolute)
			c.program_counter += 2
		case 0xdd:
			c.cmp(AbsoluteX)
			c.program_counter += 2
		case 0xd9:
			c.cmp(AbsoluteY)
			c.program_counter += 2
		case 0xc1:
			c.cmp(IndirectX)
			c.program_counter++
		case 0xd1:
			c.cmp(IndirectY)
			c.program_counter++

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
		memory:          make([]uint8, 0xFFFF),
		//memory takes default 0 with size defined
	}
}
