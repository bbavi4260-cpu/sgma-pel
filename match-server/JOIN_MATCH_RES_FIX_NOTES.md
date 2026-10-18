# JoinMatchRes protocol alignment fix

## Evidence from `sigma.cs`

The decompiled `message.LGIGCGIDOKP` declaration gives this order:

1. `MBBDNIBDPME` (`PHIFIIFMJKP`)
2. `DGCJANAMDBJ`, `LKNFCCLNBCO`, `KLGIHHCLFII` (`ulong` each)
3. `EGOMKONIBMP` (`int`), `KHEFHJNNLIL` (`byte`), `HINHHMCOFLD` (`uint`)
4. `FMMDBCOBGDM` (`ushort`), **`EOKOPECMPDF` (`long`)**
5. `CKAAOCLHHMM` (`BIOFIHHONMO` enum), `DPOPAMPEKGA` (`JFDAPOOOPIO` enum)
6. `MDGGALNBIKP`, the three lists, room-setting fields and booleans
7. `PGHKLEAHENK` (`EKHEMGJLHDC`), then outer `GHEACGCMOHK`, `IHCCLCALHMF`, `IENGJPDGBPA`

`EKHEMGJLHDC` contains two strings, a bool, a `List<byte>`, and its own trailing bool. The previous Go writer omitted the 64-bit `EOKOPECMPDF` before the two enum fields and omitted the final outer bool and byte. The missing 64-bit field shifted the subsequent payload, making the client read a non-length value as a string length in `EKHEMGJLHDC.ReadString`.

## Change

`message.JoinMatchRes` now writes the missing `U64(0)` at the exact field position and appends the outer `IHCCLCALHMF` bool and `IENGJPDGBPA` byte after the nested message.

A regression test checks the message length and important field offsets. It passes with Go 1.23 in an isolated module containing the `message` package. Full project tests could not be run in this environment because the project requires Go 1.24 and downloading that toolchain is blocked by unavailable network access. The real-client join still needs in-game verification.
