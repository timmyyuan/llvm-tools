package ir_generator

import (
	"encoding/json"
	"log"
	"strings"
	"testing"
)

func TestCompilerDatabase_EmitLLVM(t *testing.T) {
	linuxCase := `
[
  {
    "command": "gcc -Wp,-MMD,scripts/kconfig/.confdata.o.d -Wall -Wmissing-prototypes -Wstrict-prototypes -O2 -fomit-frame-pointer -std=gnu11 -Wdeclaration-after-statement -c -o scripts/kconfig/confdata.o scripts/kconfig/confdata.c",
    "directory": "/home/yuanting/linux/linux-6.2.8",
    "file": "/home/yuanting/linux/linux-6.2.8/scripts/kconfig/confdata.c"
  },
  {
    "command": "gcc -Wp,-MMD,scripts/kconfig/.util.o.d -Wall -Wmissing-prototypes -Wstrict-prototypes -O2 -fomit-frame-pointer -std=gnu11 -Wdeclaration-after-statement -c -o scripts/kconfig/util.o scripts/kconfig/util.c",
    "directory": "/home/yuanting/linux/linux-6.2.8",
    "file": "/home/yuanting/linux/linux-6.2.8/scripts/kconfig/util.c"
  }
]
`
	expect := `
[
    {
        "directory": "/home/yuanting/linux/linux-6.2.8",
        "command": "clang -Wp,-MMD,scripts/kconfig/.confdata.o.d -Wall -Wmissing-prototypes -Wstrict-prototypes -O0 -fomit-frame-pointer -std=gnu11 -Wdeclaration-after-statement -emit-llvm -g -Wno-error=all -Wno-shift-count-negative -Wno-division-by-zero -fno-inline-functions -Wno-ignored-optimization-argument -Xclang -disable-O0-optnone -c -o scripts/kconfig/confdata.bc scripts/kconfig/confdata.c",
        "file": "/home/yuanting/linux/linux-6.2.8/scripts/kconfig/confdata.c"
    },
    {
        "directory": "/home/yuanting/linux/linux-6.2.8",
        "command": "clang -Wp,-MMD,scripts/kconfig/.util.o.d -Wall -Wmissing-prototypes -Wstrict-prototypes -O0 -fomit-frame-pointer -std=gnu11 -Wdeclaration-after-statement -emit-llvm -g -Wno-error=all -Wno-shift-count-negative -Wno-division-by-zero -fno-inline-functions -Wno-ignored-optimization-argument -Xclang -disable-O0-optnone -c -o scripts/kconfig/util.bc scripts/kconfig/util.c",
        "file": "/home/yuanting/linux/linux-6.2.8/scripts/kconfig/util.c"
    }
]
`
	d := &CompilerDatabase{}
	if err := json.Unmarshal([]byte(linuxCase), &d.Commands); err != nil {
		log.Fatalln(err)
	}

	d.EmitLLVM("clang")
	result, err := json.MarshalIndent(d.Commands, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}

	if strings.TrimSpace(string(expect)) != strings.TrimSpace(string(result)) {
		t.Fail()
	}
}
