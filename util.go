package winpty

import (
    "unicode/utf16"
    "unsafe"
)

func UTF16PtrToString(p *uint16) string {
    var (
        sizeTest    uint16
        finalStr    []uint16        = make([]uint16, 0)
    )
    for {
        if *p == uint16(0) {
            break
        }

        finalStr = append(finalStr, *p)
        p = (*uint16)(unsafe.Pointer( uintptr(unsafe.Pointer(p)) + unsafe.Sizeof(sizeTest) ))
    }
    return string(utf16.Decode(finalStr[0:]))
}
