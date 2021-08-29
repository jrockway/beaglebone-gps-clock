all: CLOCK-00A0.dtbo matrix-cross

CLOCK-00A0.dtbo: CLOCK-00A0.dts
	cp CLOCK-00A0.dts bb.org-overlays/src/arm
	make -C bb.org-overlays src/arm/CLOCK-00A0.dtbo
	rm -f bb.org-overlays/src/arm/CLOCK-00A0.dts
	cp bb.org-overlays/src/arm/CLOCK-00A0.dtbo .

display-clock:
	make -C display-clock

matrix.arm: matrix/*.go go.mod go.sum
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o matrix.arm ./matrix

clean:
	rm -f *.dtbo
	make -C display-clock clean
	rm -f bb.org-overlays/src/arm/CLOCK-00A0.dts
	make -C bb.org-overlays clean
	make -C tracker/tracker clean

.PHONY: all clean display-clock
