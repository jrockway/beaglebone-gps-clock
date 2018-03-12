all: CLOCK-00A0.dtbo

CLOCK-00A0.dtbo: CLOCK-00A0.dts
	cp CLOCK-00A0.dts bb.org-overlays/src/arm
	make -C bb.org-overlays src/arm/CLOCK-00A0.dtbo
	cp bb.org-overlays/src/arm/CLOCK-00A0.dtbo .

clean:
	rm *.dtbo

.PHONY: all clean
