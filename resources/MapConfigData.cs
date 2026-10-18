using GCommon;
using Il2CppDummyDll;

namespace COW;

[Token(Token = "0x20001A0")]
public class MapConfigData : CSVBaseData
{
	[Token(Token = "0x4000A18")]
	public const uint DEFAULT_MAP_ID = 1u;

	[Token(Token = "0x4000A19")]
	[FieldOffset(Offset = "0x8")]
	public string MapName;

	[Token(Token = "0x4000A1A")]
	[FieldOffset(Offset = "0xC")]
	public string WaitingMapName;

	[Token(Token = "0x4000A1B")]
	[FieldOffset(Offset = "0x10")]
	public string MiniMapLocKey;

	[Token(Token = "0x4000A1C")]
	[FieldOffset(Offset = "0x14")]
	public string MapBgSpriteName;

	[Token(Token = "0x4000A1D")]
	[FieldOffset(Offset = "0x18")]
	public string OpeningSoundID;

	[Token(Token = "0x4000A1E")]
	[FieldOffset(Offset = "0x1C")]
	public uint ConfigID;

	[Token(Token = "0x4000A1F")]
	[FieldOffset(Offset = "0x20")]
	public uint MapID;

	[Token(Token = "0x4000A20")]
	[FieldOffset(Offset = "0x24")]
	public uint ModeID;

	[Token(Token = "0x4000A21")]
	[FieldOffset(Offset = "0x28")]
	public bool IsCraftland;

	[Token(Token = "0x4000A22")]
	[FieldOffset(Offset = "0x2C")]
	public ResourceID MapResourceID;

	[Token(Token = "0x4000A23")]
	[FieldOffset(Offset = "0x30")]
	public ResourceID MapABTexRefInfoID;

	[Token(Token = "0x4000A24")]
	[FieldOffset(Offset = "0x34")]
	public ResourceID WaitingMapResourceID;

	[Token(Token = "0x4000A25")]
	[FieldOffset(Offset = "0x38")]
	public ResourceID ModeMapResourceID;

	[Token(Token = "0x4000A26")]
	[FieldOffset(Offset = "0x3C")]
	public ResourceID ModeDepAbFakeResID;

	[Token(Token = "0x4000A27")]
	[FieldOffset(Offset = "0x40")]
	public ResourceID BigMapBgResourceID;

	[Token(Token = "0x4000A28")]
	[FieldOffset(Offset = "0x44")]
	public ResourceID ShowPositionResourceID;

	[Token(Token = "0x4000A29")]
	[FieldOffset(Offset = "0x48")]
	public ResourceID[] ModeDepConfigResIDs;

	[Token(Token = "0x4000A2A")]
	[FieldOffset(Offset = "0x4C")]
	public bool IsNoWaitingMap;

	[Token(Token = "0x4000A2B")]
	[FieldOffset(Offset = "0x4D")]
	public bool IsNoSafeZone;

	[Token(Token = "0x4000A2C")]
	[FieldOffset(Offset = "0x4E")]
	public bool IsNoJumping;

	[Token(Token = "0x4000A2D")]
	public const uint PRE_STUDY_MODE_ID_MIN = 230u;

	[Token(Token = "0x4000A2E")]
	public const uint PRE_STUDY_MODE_ID_MAX = 255u;

	[Token(Token = "0x600090A")]
	[Address(RVA = "0x1534458", Offset = "0x1534458", VA = "0x1534458")]
	public MapConfigData()
	{
	}

	[Token(Token = "0x600090B")]
	[Address(RVA = "0x153459C", Offset = "0x153459C", VA = "0x153459C", Slot = "5")]
	public override string GetPrimaryKey()
	{
		return null;
	}

	[Token(Token = "0x600090C")]
	[Address(RVA = "0x1534600", Offset = "0x1534600", VA = "0x1534600", Slot = "4")]
	public override void ParseData(long index, int fieldCount, string[] headers, string[] values)
	{
	}

	[Token(Token = "0x600090D")]
	[Address(RVA = "0x153519C", Offset = "0x153519C", VA = "0x153519C")]
	public static uint MakeConfigID(uint map_id, uint mode_id)
	{
		return default(uint);
	}

	[Token(Token = "0x600090E")]
	[Address(RVA = "0x1535204", Offset = "0x1535204", VA = "0x1535204")]
	public static uint GetMapIDFromConfigID(uint configID)
	{
		return default(uint);
	}

	[Token(Token = "0x600090F")]
	[Address(RVA = "0x153526C", Offset = "0x153526C", VA = "0x153526C")]
	public static uint GetModeIDFromConfigID(uint configID)
	{
		return default(uint);
	}
}
