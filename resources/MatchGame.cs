using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using COW.GamePlay;
using GCommon;
using Il2CppDummyDll;
using UnityEngine;
using message;

namespace COW;

[Token(Token = "0x20003C1")]
internal abstract class MatchGame : COWGameBase
{
	[Token(Token = "0x4003857")]
	[FieldOffset(Offset = "0x3C")]
	protected NFJPHMKKEBF m_Match;

	[Token(Token = "0x4003858")]
	[FieldOffset(Offset = "0x40")]
	protected GameModeSetting m_GameModeSetting;

	[Token(Token = "0x4003859")]
	[FieldOffset(Offset = "0x44")]
	protected uint m_ReturnToLobbyRequest;

	[Token(Token = "0x400385A")]
	[FieldOffset(Offset = "0x48")]
	protected IFNFNDJLKIF m_LevelObjectManager;

	[Token(Token = "0x400385B")]
	[FieldOffset(Offset = "0x4C")]
	protected ADMPIKLECJF m_MetaManager;

	[Token(Token = "0x400385C")]
	[FieldOffset(Offset = "0x50")]
	protected Dictionary<uint, ReplicationEntity> m_ReplicationEntitis;

	[Token(Token = "0x400385D")]
	[FieldOffset(Offset = "0x54")]
	protected IGRIDataPool m_GRIDataPool;

	[Token(Token = "0x400385E")]
	[FieldOffset(Offset = "0x58")]
	protected CameraControllerManager m_CameraControllerManager;

	[Token(Token = "0x400385F")]
	[FieldOffset(Offset = "0x5C")]
	protected uint m_QuitMatchRequest;

	[Token(Token = "0x4003860")]
	[FieldOffset(Offset = "0x60")]
	protected bool mUseAB;

	[Token(Token = "0x4003861")]
	[FieldOffset(Offset = "0x61")]
	protected bool m_isSendMsg;

	[Token(Token = "0x4003862")]
	[FieldOffset(Offset = "0x64")]
	protected string mSceneABPath;

	[Token(Token = "0x4003863")]
	[FieldOffset(Offset = "0x68")]
	protected string mStreamSceneAbPath;

	[Token(Token = "0x4003864")]
	[FieldOffset(Offset = "0x6C")]
	protected InGameCutsceneManager m_InGameCutsceneManager;

	[Token(Token = "0x4003865")]
	[FieldOffset(Offset = "0x70")]
	protected AssetBundle m_streamSceneAB;

	[Token(Token = "0x4003866")]
	[FieldOffset(Offset = "0x74")]
	protected GameObject m_UmaDcs;

	[Token(Token = "0x4003867")]
	[FieldOffset(Offset = "0x78")]
	protected WolfTrapManager m_WolfTrapManager;

	[Token(Token = "0x4003868")]
	[FieldOffset(Offset = "0x7C")]
	protected HashSet<ResourceID> m_UnUsedItemResourceList;

	[Token(Token = "0x4003869")]
	[FieldOffset(Offset = "0x80")]
	protected Dictionary<ResourceID, ResourceManager.PreloadResourceInfo> m_PreloadResourcePoolInitDic;

	[Token(Token = "0x600106F")]
	[Address(RVA = "0x153CC50", Offset = "0x153CC50", VA = "0x153CC50")]
	protected MatchGame()
	{
	}

	[Token(Token = "0x6001070")]
	[Address(RVA = "0x153CDE0", Offset = "0x153CDE0", VA = "0x153CDE0")]
	public NFJPHMKKEBF GetMatch()
	{
		return null;
	}

	[Token(Token = "0x6001071")]
	[Address(RVA = "0x153CE38", Offset = "0x153CE38", VA = "0x153CE38", Slot = "45")]
	public virtual bool IsOnlineGame()
	{
		return default(bool);
	}

	[Token(Token = "0x6001072")]
	[Address(RVA = "0x153CE90", Offset = "0x153CE90", VA = "0x153CE90", Slot = "28")]
	public override bool NotUsingAB()
	{
		return default(bool);
	}

	[Token(Token = "0x6001073")]
	[Address(RVA = "0x153CEFC", Offset = "0x153CEFC", VA = "0x153CEFC", Slot = "29")]
	public override void SetSceneABInfo(AssetBundle ab, bool useAB, string abPath, string streamABPath)
	{
	}

	[Token(Token = "0x6001074")]
	[Address(RVA = "0x153CF8C", Offset = "0x153CF8C", VA = "0x153CF8C", Slot = "32")]
	public override string GetSceneStreamABPath()
	{
		return null;
	}

	[Token(Token = "0x6001075")]
	[Address(RVA = "0x153CFE4", Offset = "0x153CFE4", VA = "0x153CFE4", Slot = "30")]
	public override AssetBundle GetStreamSceneAB()
	{
		return null;
	}

	[Token(Token = "0x6001076")]
	[Address(RVA = "0x153D03C", Offset = "0x153D03C", VA = "0x153D03C", Slot = "31")]
	public override byte[] GetStreamSceneABTexRefInfo()
	{
		return null;
	}

	[Token(Token = "0x6001077")]
	[Address(RVA = "0x153D460", Offset = "0x153D460", VA = "0x153D460", Slot = "19")]
	public override Type GetUISceneType()
	{
		return null;
	}

	[Token(Token = "0x6001078")]
	[Address(RVA = "0x153D53C", Offset = "0x153D53C", VA = "0x153D53C", Slot = "8")]
	protected override void OnAwake()
	{
	}

	[Token(Token = "0x6001079")]
	[Address(RVA = "0x153D89C", Offset = "0x153D89C", VA = "0x153D89C")]
	private void UIRootOnAwake(UIRoot uiroot)
	{
	}

	[Token(Token = "0x600107A")]
	[Address(RVA = "0x153DA04", Offset = "0x153DA04", VA = "0x153DA04", Slot = "46")]
	protected virtual void InitGRIData()
	{
	}

	[Token(Token = "0x600107B")]
	[Address(RVA = "0x153DE08", Offset = "0x153DE08", VA = "0x153DE08", Slot = "9")]
	protected override void OnSceneLoaded()
	{
	}

	[Token(Token = "0x600107C")]
	[Address(RVA = "0x153ED2C", Offset = "0x153ED2C", VA = "0x153ED2C", Slot = "11")]
	protected override void OnUnInit()
	{
	}

	[Token(Token = "0x600107D")]
	[Address(RVA = "0x153EEB8", Offset = "0x153EEB8", VA = "0x153EEB8", Slot = "12")]
	protected override void OnDestroy()
	{
	}

	[Token(Token = "0x600107E")]
	[Address(RVA = "0x153F1A0", Offset = "0x153F1A0", VA = "0x153F1A0")]
	public bool ShowGameOpeingAnimation()
	{
		return default(bool);
	}

	[Token(Token = "0x600107F")]
	[Address(RVA = "0x153F2E4", Offset = "0x153F2E4", VA = "0x153F2E4")]
	public bool GameOpeningMovieAnimationEnable()
	{
		return default(bool);
	}

	[Token(Token = "0x6001080")]
	[Address(RVA = "0x153F424", Offset = "0x153F424", VA = "0x153F424")]
	private void OnReturnToLobby(object[] data)
	{
	}

	[Token(Token = "0x6001081")]
	[Address(RVA = "0x153FC54", Offset = "0x153FC54", VA = "0x153FC54", Slot = "47")]
	protected virtual void OnLeaveMatchGame()
	{
	}

	[Token(Token = "0x6001082")]
	[Address(RVA = "0x153FCA8", Offset = "0x153FCA8", VA = "0x153FCA8")]
	public IFNFNDJLKIF GetLevelObjectManager()
	{
		return null;
	}

	[Token(Token = "0x6001083")]
	[Address(RVA = "0x153FD00", Offset = "0x153FD00", VA = "0x153FD00")]
	public ADMPIKLECJF GetMetaManager()
	{
		return null;
	}

	[Token(Token = "0x6001084")]
	[Address(RVA = "0x153FD58", Offset = "0x153FD58", VA = "0x153FD58")]
	public WolfTrapManager GetWolfTrapManager()
	{
		return null;
	}

	[Token(Token = "0x6001085")]
	[Address(RVA = "0x153FE20", Offset = "0x153FE20", VA = "0x153FE20", Slot = "48")]
	public virtual GameModeSetting GetGameModeSetting()
	{
		return null;
	}

	[Token(Token = "0x6001086")]
	[Address(RVA = "0x153FFA4", Offset = "0x153FFA4", VA = "0x153FFA4")]
	public CameraControllerManager GetCameraControllerManager()
	{
		return null;
	}

	[Token(Token = "0x6001087")]
	[Address(RVA = "0x153FFFC", Offset = "0x153FFFC", VA = "0x153FFFC")]
	public void RegisterRepEntity(ReplicationEntity entity)
	{
	}

	[Token(Token = "0x6001088")]
	[Address(RVA = "0x154014C", Offset = "0x154014C", VA = "0x154014C")]
	public void UnRegisterRepEntity(ReplicationEntity entity)
	{
	}

	[Token(Token = "0x6001089")]
	[Address(RVA = "0x1540238", Offset = "0x1540238", VA = "0x1540238")]
	public ReplicationEntity GetRepEntity(uint id)
	{
		return null;
	}

	[Token(Token = "0x600108A")]
	[Address(RVA = "0x1540328", Offset = "0x1540328", VA = "0x1540328")]
	public void OnSyncReplicationData(FastBinaryReader reader)
	{
	}

	[Token(Token = "0x600108B")]
	[Address(RVA = "0x1540454", Offset = "0x1540454", VA = "0x1540454", Slot = "49")]
	protected virtual void OnAlivePlayerCountChanged(byte oldValue, byte newValue)
	{
	}

	[Token(Token = "0x600108C")]
	[Address(RVA = "0x1540598", Offset = "0x1540598", VA = "0x1540598")]
	protected void AddUmaDC()
	{
	}

	[Token(Token = "0x600108D")]
	[Address(RVA = "0x15409F0", Offset = "0x15409F0", VA = "0x15409F0")]
	public void ReleaseUmaDc()
	{
	}

	[Token(Token = "0x600108E")]
	[Address(RVA = "0x1540B10", Offset = "0x1540B10", VA = "0x1540B10")]
	public void EnsureUmaDc()
	{
	}

	[Token(Token = "0x600108F")]
	[Address(RVA = "0x1540BF4", Offset = "0x1540BF4", VA = "0x1540BF4")]
	public void OnRequestQuitMatchRes(OOCKDNJDDLF msg)
	{
	}

	[Token(Token = "0x6001090")]
	[Address(RVA = "0x1540D20", Offset = "0x1540D20", VA = "0x1540D20")]
	public bool IsLocalPlayerAlive()
	{
		return default(bool);
	}

	[Token(Token = "0x6001091")]
	[Address(RVA = "0x1540E58", Offset = "0x1540E58", VA = "0x1540E58")]
	public bool IsLocalPlayerPendingRevive()
	{
		return default(bool);
	}

	[Token(Token = "0x6001092")]
	[Address(RVA = "0x1540FD4", Offset = "0x1540FD4", VA = "0x1540FD4")]
	public bool IsLocalObserverFinishedMatch()
	{
		return default(bool);
	}

	[Token(Token = "0x6001093")]
	[Address(RVA = "0x15413D0", Offset = "0x15413D0", VA = "0x15413D0")]
	public bool IsLocalPlayerFinishedMatch()
	{
		return default(bool);
	}

	[Token(Token = "0x6001094")]
	[Address(RVA = "0x15415A8", Offset = "0x15415A8", VA = "0x15415A8")]
	public void EndObserver()
	{
	}

	[Token(Token = "0x6001095")]
	[Address(RVA = "0x15416D4", Offset = "0x15416D4", VA = "0x15416D4")]
	public void SendQuitRequest(EQuitGameReason reason = EQuitGameReason.Normal)
	{
	}

	[Token(Token = "0x6001096")]
	[Address(RVA = "0x154180C", Offset = "0x154180C", VA = "0x154180C", Slot = "50")]
	public virtual void SendQuitFakeBotRequest()
	{
	}

	[Token(Token = "0x6001097")]
	[Address(RVA = "0x1541860", Offset = "0x1541860", VA = "0x1541860")]
	public void RequestShowMatchResult()
	{
	}

	[Token(Token = "0x6001098")]
	[Address(RVA = "0x154197C", Offset = "0x154197C", VA = "0x154197C")]
	public void RoomSpectatorRequestQuitMatch()
	{
	}

	[Token(Token = "0x6001099")]
	[Address(RVA = "0x1541BF0", Offset = "0x1541BF0", VA = "0x1541BF0")]
	public void RequestQuitMatch(EQuitGameReason reason = EQuitGameReason.Normal)
	{
	}

	[Token(Token = "0x600109A")]
	[Address(RVA = "0x1541F14", Offset = "0x1541F14", VA = "0x1541F14", Slot = "51")]
	public virtual void OnQuitMatchToLobby()
	{
	}

	[Token(Token = "0x600109B")]
	[Address(RVA = "0x1542500", Offset = "0x1542500", VA = "0x1542500", Slot = "34")]
	public override bool ShouldClearPreloadAssets(Type nextGameType)
	{
		return default(bool);
	}

	[Token(Token = "0x600109C")]
	[Address(RVA = "0x1542610", Offset = "0x1542610", VA = "0x1542610")]
	protected ResourceID[] GetPreloadResourceIDByGameMode()
	{
		return null;
	}

	[Token(Token = "0x600109D")]
	[Address(RVA = "0x15427C0", Offset = "0x15427C0", VA = "0x15427C0")]
	protected ResourceID[] GetPreloadPlayerAnimatorsResourceID()
	{
		return null;
	}

	[Token(Token = "0x600109E")]
	[Address(RVA = "0x1542998", Offset = "0x1542998", VA = "0x1542998")]
	protected void FilterUnUsedItemRes(ResourceID[] res, ref List<ResourceID> ret)
	{
	}

	[Token(Token = "0x600109F")]
	[Address(RVA = "0x1542B6C", Offset = "0x1542B6C", VA = "0x1542B6C", Slot = "20")]
	public override ResourceID[] GetPreloadResourceID()
	{
		return null;
	}

	[Token(Token = "0x60010A0")]
	[Address(RVA = "0x1542E44", Offset = "0x1542E44", VA = "0x1542E44", Slot = "52")]
	public virtual bool NeedPoolInstantRes()
	{
		return default(bool);
	}

	[Token(Token = "0x60010A1")]
	[Address(RVA = "0x153E67C", Offset = "0x153E67C", VA = "0x153E67C")]
	protected void PrePoolInstantRes()
	{
	}

	[Token(Token = "0x60010A2")]
	[Address(RVA = "0x1542E9C", Offset = "0x1542E9C", VA = "0x1542E9C")]
	public InGameCutsceneManager GetInGameCutsceneManager()
	{
		return null;
	}

	[Token(Token = "0x60010A3")]
	[Address(RVA = "0x1542EF4", Offset = "0x1542EF4", VA = "0x1542EF4")]
	public void RegisterInGameCutsceneManager(InGameCutsceneManager manager)
	{
	}

	[Token(Token = "0x60010A4")]
	[Address(RVA = "0x1542F54", Offset = "0x1542F54", VA = "0x1542F54", Slot = "14")]
	protected override void OnInputUpdate()
	{
	}

	[Token(Token = "0x60010A5")]
	[Address(RVA = "0x1542FA8", Offset = "0x1542FA8", VA = "0x1542FA8", Slot = "53")]
	public virtual Player CreatePlayer(Transform container, ulong userID, ulong serviceGroupID, IHAAMHPPLMG playerID, bool isAI = false, [Optional] object syncInfo)
	{
		return null;
	}

	[Token(Token = "0x60010A6")]
	[Address(RVA = "0x1543150", Offset = "0x1543150", VA = "0x1543150", Slot = "54")]
	public virtual void OnAddPlayer(Player p)
	{
	}

	[Token(Token = "0x60010A7")]
	[Address(RVA = "0x15431AC", Offset = "0x15431AC", VA = "0x15431AC", Slot = "55")]
	public virtual bool IsAimingEnemyInGameMode(bool isAimingEnemy, AEPAPLNNAHF player, OPILIBBOEAC aimTarget)
	{
		return default(bool);
	}

	[Token(Token = "0x60010A8")]
	[Address(RVA = "0x154322C", Offset = "0x154322C", VA = "0x154322C", Slot = "56")]
	public virtual bool IsGameAllowFlight()
	{
		return default(bool);
	}

	[Token(Token = "0x60010A9")]
	[Address(RVA = "0x1543284", Offset = "0x1543284", VA = "0x1543284", Slot = "57")]
	public virtual bool IsFriendFire(Player attacker, OPILIBBOEAC victim, bool friendDmgEnable, OOIPMACFIFL weaponData)
	{
		return default(bool);
	}

	[Token(Token = "0x60010AA")]
	[Address(RVA = "0x1543664", Offset = "0x1543664", VA = "0x1543664")]
	private void _003COnReturnToLobby_003Em__0()
	{
	}

	[Token(Token = "0x60010AB")]
	[Address(RVA = "0x15437DC", Offset = "0x15437DC", VA = "0x15437DC")]
	private void _003CRoomSpectatorRequestQuitMatch_003Em__1()
	{
	}

	[Token(Token = "0x60010AC")]
	[Address(RVA = "0x15437EC", Offset = "0x15437EC", VA = "0x15437EC")]
	private void _003CRequestQuitMatch_003Em__2()
	{
	}

	[Token(Token = "0x60010AD")]
	[Address(RVA = "0x15437FC", Offset = "0x15437FC", VA = "0x15437FC")]
	public bool _003C_003EiFixBaseProxy_NotUsingAB()
	{
		return default(bool);
	}

	[Token(Token = "0x60010AE")]
	[Address(RVA = "0x1543804", Offset = "0x1543804", VA = "0x1543804")]
	public void _003C_003EiFixBaseProxy_SetSceneABInfo(AssetBundle P0, bool P1, string P2, string P3)
	{
	}

	[Token(Token = "0x60010AF")]
	[Address(RVA = "0x1543828", Offset = "0x1543828", VA = "0x1543828")]
	public string _003C_003EiFixBaseProxy_GetSceneStreamABPath()
	{
		return null;
	}

	[Token(Token = "0x60010B0")]
	[Address(RVA = "0x1543830", Offset = "0x1543830", VA = "0x1543830")]
	public AssetBundle _003C_003EiFixBaseProxy_GetStreamSceneAB()
	{
		return null;
	}

	[Token(Token = "0x60010B1")]
	[Address(RVA = "0x1543838", Offset = "0x1543838", VA = "0x1543838")]
	public byte[] _003C_003EiFixBaseProxy_GetStreamSceneABTexRefInfo()
	{
		return null;
	}

	[Token(Token = "0x60010B2")]
	[Address(RVA = "0x1543840", Offset = "0x1543840", VA = "0x1543840")]
	public new void _003C_003EiFixBaseProxy_OnAwake()
	{
	}

	[Token(Token = "0x60010B3")]
	[Address(RVA = "0x1543848", Offset = "0x1543848", VA = "0x1543848")]
	public new void _003C_003EiFixBaseProxy_OnSceneLoaded()
	{
	}

	[Token(Token = "0x60010B4")]
	[Address(RVA = "0x1543850", Offset = "0x1543850", VA = "0x1543850")]
	public void _003C_003EiFixBaseProxy_OnUnInit()
	{
	}

	[Token(Token = "0x60010B5")]
	[Address(RVA = "0x1543858", Offset = "0x1543858", VA = "0x1543858")]
	public new void _003C_003EiFixBaseProxy_OnDestroy()
	{
	}

	[Token(Token = "0x60010B6")]
	[Address(RVA = "0x1543860", Offset = "0x1543860", VA = "0x1543860")]
	public bool _003C_003EiFixBaseProxy_ShouldClearPreloadAssets(Type P0)
	{
		return default(bool);
	}

	[Token(Token = "0x60010B7")]
	[Address(RVA = "0x1543868", Offset = "0x1543868", VA = "0x1543868")]
	public ResourceID[] _003C_003EiFixBaseProxy_GetPreloadResourceID()
	{
		return null;
	}

	[Token(Token = "0x60010B8")]
	[Address(RVA = "0x1543870", Offset = "0x1543870", VA = "0x1543870")]
	public void _003C_003EiFixBaseProxy_OnInputUpdate()
	{
	}
}
