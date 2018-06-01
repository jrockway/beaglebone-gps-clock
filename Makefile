all: CLOCK-00A0.dtbo display-clock

CLOCK-00A0.dtbo: CLOCK-00A0.dts
	cp CLOCK-00A0.dts bb.org-overlays/src/arm
	make -C bb.org-overlays src/arm/CLOCK-00A0.dtbo
	rm -f bb.org-overlays/src/arm/CLOCK-00A0.dts
	cp bb.org-overlays/src/arm/CLOCK-00A0.dtbo .

display-clock:
	make -C display-clock

clean:
	rm -f *.dtbo
	make -C display-clock clean
	rm -f bb.org-overlays/src/arm/CLOCK-00A0.dts
	make -C bb.org-overlays clean
	make -C tracker/tracker clean

.PHONY: all clean display-clock
