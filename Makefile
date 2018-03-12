all: CLOCK-00A0.dtbo display-clock

CLOCK-00A0.dtbo: CLOCK-00A0.dts
	cp CLOCK-00A0.dts bb.org-overlays/src/arm
	make -C bb.org-overlays src/arm/CLOCK-00A0.dtbo
	cp bb.org-overlays/src/arm/CLOCK-00A0.dtbo .

display-clock:
	make -C display-clock

clean:
	rm *.dtbo
	make -C display-clock clean
	make -C bb.org-overlays clean

.PHONY: all clean display-clock
