PROG=eth
TARG=nucleo-h723zg

OPT=z

TGOFLAGS=\
	-target $(TARG)\
	-gc leaking \
	-size short\
	-opt $(OPT)\

all: $(PROG).hex

%.hex: *.go
	tinygo build -o $@ $(TGOFLAGS) .

%.elf: *.go
	tinygo build -o $@ $(TGOFLAGS) .

flash: *.go
	tinygo flash $(TGOFLAGS) .

objdump: $(PROG).elf
	arm-none-eabi-objdump -S $< > $(PROG).objdump

clean:
	rm -f $(PROG).hex

.PHONY: \
	all \
	flash \
	clean \
