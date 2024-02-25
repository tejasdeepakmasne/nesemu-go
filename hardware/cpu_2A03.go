package hardware

// Constants for stack start address and stack reset value
// The reason the NES stack ends at 253 bytes (0x01FD) rather than 256 bytes (0x01FF) is due to a hardware limitation.
// The top three addresses (0x01FD, 0x01FE, and 0x01FF) are reserved for the NES's interrupt vector table.
const STACK_START uint16 = 0x0100
const STACK_RESET uint8 = 0xfd

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

func (c *CPU) load(instructions []uint8) {
	for i, val := range instructions {
		c.memory[8000+i] = val
	}
	c.mem_write_16(0xFFFC, 0x8000) // set the reset vector https://en.wikipedia.org/wiki/Reset_vector
}

func (c *CPU) reset() {

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
