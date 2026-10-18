using GCommon;
using Il2CppDummyDll;

namespace COW;

[Token(Token = "0x20001A1")]
public class MapNameConfigData : CSVBaseData
{
	[Token(Token = "0x4000A2F")]
	[FieldOffset(Offset = "0x8")]
	public uint MapID;

	[Token(Token = "0x4000A30")]
	[FieldOffset(Offset = "0xC")]
	public string MapName;

	[Token(Token = "0x4000A31")]
	[FieldOffset(Offset = "0x10")]
	public string MapSprite;

	[Token(Token = "0x6000910")]
	[Address(RVA = "0x15356F8", Offset = "0x15356F8", VA = "0x15356F8")]
	public MapNameConfigData()
	{
	}

	[Token(Token = "0x6000911")]
	[Address(RVA = "0x153577C", Offset = "0x153577C", VA = "0x153577C", Slot = "5")]
	public override string GetPrimaryKey()
	{
		return null;
	}

	[Token(Token = "0x6000912")]
	[Address(RVA = "0x15357E0", Offset = "0x15357E0", VA = "0x15357E0", Slot = "4")]
	public override void ParseData(long index, int fieldCount, string[] headers, string[] values)
	{
	}
}
