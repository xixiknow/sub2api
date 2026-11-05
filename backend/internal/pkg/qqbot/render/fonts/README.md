# Fonts

`NotoSansSC-Regular.ttf` / `NotoSansSC-Bold.ttf` are static instances (wght 400 / 700)
of Noto Sans SC (© Adobe, SIL Open Font License 1.1, https://openfontlicense.org),
subset to ASCII + GB2312 + a few UI symbols to keep the binary small.

Regenerate with fontTools: `varLib.instancer.instantiateVariableFont(vf, {"wght": 400|700})`
then `fontTools.subset` with the same character set.
