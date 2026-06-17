package ios

import (
	"bytes"
	"debug/dwarf"
	"debug/macho"
	"fmt"
)

const cmdUUID = 0x1b

type SliceInfo struct {
	UUID string
	Arch string
}

func isFatMagic(data []byte) bool {
	return len(data) >= 4 &&
		(data[0] == 0xca && data[1] == 0xfe && data[2] == 0xba && data[3] == 0xbe ||
			data[0] == 0xbe && data[1] == 0xba && data[2] == 0xfe && data[3] == 0xca)
}

func IsMachO(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	switch {
	case data[0] == 0xfe && data[1] == 0xed && data[2] == 0xfa && (data[3] == 0xce || data[3] == 0xcf):
		return true
	case (data[0] == 0xce || data[0] == 0xcf) && data[1] == 0xfa && data[2] == 0xed && data[3] == 0xfe:
		return true
	}
	return isFatMagic(data)
}

func machoFiles(data []byte) ([]*macho.File, func(), error) {
	if len(data) < 4 {
		return nil, func() {}, fmt.Errorf("not a Mach-O file: too short")
	}
	if isFatMagic(data) {
		fat, err := macho.NewFatFile(bytes.NewReader(data))
		if err != nil {
			return nil, func() {}, fmt.Errorf("reading fat Mach-O: %w", err)
		}
		files := make([]*macho.File, len(fat.Arches))
		for i := range fat.Arches {
			files[i] = fat.Arches[i].File
		}
		return files, func() { fat.Close() }, nil
	}
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, func() {}, fmt.Errorf("not a Mach-O file: %w", err)
	}
	return []*macho.File{f}, func() { f.Close() }, nil
}

func ReadUUIDs(data []byte) ([]SliceInfo, error) {
	files, done, err := machoFiles(data)
	if err != nil {
		return nil, err
	}
	defer done()
	var out []SliceInfo
	for _, f := range files {
		uuid := readMachoUUID(f)
		if uuid == "" {
			continue
		}
		out = append(out, SliceInfo{UUID: uuid, Arch: archFromCPU(f.Cpu)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no Mach-O slice with an LC_UUID")
	}
	return out, nil
}

func dwarfForSlice(data []byte, uuid, arch string) (*dwarf.Data, uint64, string, error) {
	files, done, err := machoFiles(data)
	if err != nil {
		return nil, 0, "", err
	}
	defer done()

	uuid = NormalizeUUID(uuid)
	want := NormalizeArch(arch)
	var chosen *macho.File
	if uuid != "" {
		for _, f := range files {
			if readMachoUUID(f) == uuid {
				chosen = f
				break
			}
		}
	}
	if chosen == nil {
		for _, f := range files {
			if want == "" || NormalizeArch(archFromCPU(f.Cpu)) == want {
				chosen = f
				break
			}
		}
	}
	if chosen == nil && len(files) == 1 {
		chosen = files[0]
	}
	if chosen == nil {
		return nil, 0, "", fmt.Errorf("no slice matching uuid %q / arch %q in dSYM", uuid, arch)
	}
	d, err := chosen.DWARF()
	if err != nil {
		return nil, 0, "", fmt.Errorf("reading DWARF: %w", err)
	}
	var textVMAddr uint64
	if seg := chosen.Segment("__TEXT"); seg != nil {
		textVMAddr = seg.Addr
	}
	return d, textVMAddr, readMachoUUID(chosen), nil
}

func readMachoUUID(f *macho.File) string {
	for _, l := range f.Loads {
		lb, ok := l.(macho.LoadBytes)
		if !ok {
			continue
		}
		raw := lb.Raw()
		if len(raw) >= 24 && f.ByteOrder.Uint32(raw[0:4]) == cmdUUID {
			return fmt.Sprintf("%x", raw[8:24])
		}
	}
	return ""
}

func archFromCPU(c macho.Cpu) string {
	switch c {
	case macho.CpuArm64:
		return "arm64"
	case macho.CpuAmd64:
		return "x64"
	case macho.CpuArm:
		return "arm"
	case macho.Cpu386:
		return "ia32"
	}
	return fmt.Sprintf("cpu_%d", uint32(c))
}
