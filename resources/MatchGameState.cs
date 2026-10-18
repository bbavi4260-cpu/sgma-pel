using Il2CppDummyDll;

namespace COW;

[Token(Token = "0x200319E")]
public class MatchGameState
{
	[Token(Token = "0x4012DC3")]
	[FieldOffset(Offset = "0x0")]
	public static uint TrainIslandTips;

	[Token(Token = "0x4012DC4")]
	[FieldOffset(Offset = "0x4")]
	public static uint TrainIslandGoTo;

	[Token(Token = "0x4012DC5")]
	[FieldOffset(Offset = "0x8")]
	public static uint RankGameTipsOrPopWindow;

	[Token(Token = "0x4012DC6")]
	[FieldOffset(Offset = "0xC")]
	public static uint RankGameGoToOrPopWindow;

	[Token(Token = "0x4012DC7")]
	[FieldOffset(Offset = "0x10")]
	public static uint RankGameGoTo;

	[Token(Token = "0x6014DA4")]
	[Address(RVA = "0x1543878", Offset = "0x1543878", VA = "0x1543878")]
	public MatchGameState()
	{
	}
}
