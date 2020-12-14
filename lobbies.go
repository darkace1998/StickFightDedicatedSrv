package main

import (
	"errors"
	"net"
	"sync"
)

var (
	lobbies = make([]*lobby, 0)
)

type lobby struct {
	//We don't want race conditions with such a latent game
	sync.Mutex

	//Lobby settings
	MaxPlayers      int
	Health          byte
	Regen           byte
	WeaponSpawnRate byte
	Private         bool

	//Session tracker
	MapIndex   int  //The current map as indexed from lobby.Maps, -1 for lobby
	InFight    bool //If the match is in progress
	LastWinner byte //The player index of the last winner

	Players []player
	Maps    []*level
}

func newLobby() *lobby {
	//Constants
	defaultMaxPlayers := 4
	defaultMaps := []*level{
		newMapLandfall(1),
		newMapLandfall(2),
		newMapLandfall(3),
		newMapLandfall(4),
		newMapCustomOnline(2200042304),
	}

	daLobby := &lobby{
		MaxPlayers: defaultMaxPlayers,
		Players:    make([]player, defaultMaxPlayers),
		Maps:       defaultMaps,
		MapIndex:   -1,
	}

	return daLobby
}

func (l *lobby) SendTo(p *packet, dst *net.UDPAddr) {
	srv.WriteToUDP(p.AsBytes(), dst)
	//log.Debug("Sent to ", dst.String(), ": ", p.String())
}

func (l *lobby) Broadcast(p *packet, caller *net.UDPAddr) {
	//If the caller is set, make sure the Steam ID is set too
	if caller != nil {
		for _, pl := range l.Players {
			if pl.Addr != nil {
				if caller.String() == pl.Addr.String() {
					p.SteamID = pl.SteamID
					break
				}
			}
		}
	}

	//Broadcast the packet
	for _, pl := range l.Players {
		if pl.Addr != nil {
			if caller != nil {
				if caller.String() != pl.Addr.String() { //Don't send the packet to the caller
					l.SendTo(p, pl.Addr)
				}
			} else {
				l.SendTo(p, pl.Addr)
			}
		}
	}
}

func (l *lobby) GetMaxHealth() float32 {
	switch l.Health {
	case 0:
		return 100
	case 1:
		return 200
	case 2:
		return 300
	case 3:
		return 1
	case 4:
		return 25
	case 5:
		return 50
	case 6:
		return 75
	}

	return 0
}

func (l *lobby) TryStartMatch() {
	log.Info("Trying to start match...")

	if l.InFight {
		log.Warn("Can't start match when already in fight!")
		return
	}

	notReady := false
	for i := 0; i < l.MaxPlayers; i++ {
		pl := l.Players[i]
		if pl.Addr == nil {
			continue
		}
		if !pl.Status.Ready {
			notReady = true
			l.SendTo(newPacket(packetTypeStartMatch, 1, l.Players[l.GetHostIndex()].SteamID), pl.Addr)
			break
		}
	}

	if notReady {
		log.Warn("Can't start match until all players are ready!")
	} else {
		l.InFight = true
		l.Broadcast(newPacket(packetTypeStartMatch, 1, 0), nil)
		log.Info("Started match!")
	}
}

func (l *lobby) CheckWinner(playerIndex int) {
	someoneElseSurvived := false
	for i, pl := range l.Players {
		if i == playerIndex {
			continue
		}
		if pl.Addr != nil {
			if !pl.Status.Dead {
				someoneElseSurvived = true
				break
			}
		}
	}

	if !someoneElseSurvived {
		log.Info("Player ", playerIndex, " is the winner")
		l.LastWinner = byte(playerIndex)
		l.ChangeMap(-1)
	}
}

func (l *lobby) IsInLobby() bool {
	return l.MapIndex == -1
}

func (l *lobby) GetMap() *level {
	if l.MapIndex == -1 {
		return newMapLandfall(0) //Lobby map
	}
	return l.Maps[l.MapIndex]
}

func (l *lobby) ChangeMap(mapIndex int) {
	//l.Lock()
	//defer l.Unlock()

	if mapIndex < 0 || mapIndex >= len(l.Maps) {
		mapIndex = randomizer.Intn(len(l.Maps) - 1)
	}

	l.MapIndex = mapIndex

	l.UnReadyAllPlayers()
	l.InFight = false

	packetMapChange := newPacket(packetTypeMapChange, 1, 0)
	packetMapChange.Grow(2)
	packetMapChange.WriteByteNext(l.LastWinner)
	packetMapChange.WriteByteNext(l.Maps[mapIndex].Type())
	packetMapChange.Grow(int64(l.Maps[mapIndex].Size()))
	packetMapChange.WriteBytesNext(l.Maps[mapIndex].Data())

	l.Broadcast(packetMapChange, nil)
	log.Info("Changed map index to ", mapIndex, ": ", l.Maps[mapIndex])

	l.SpawnPlayers()
}

func (l *lobby) GetPlayersInLobby(excludePlayerIndex int) int {
	playerCount := 0
	for i, pl := range l.Players {
		if pl.Addr == nil {
			continue
		}
		if excludePlayerIndex == i {
			continue
		}
		playerCount++
	}
	return playerCount
}

func (l *lobby) IsPlayerReady(playerIndex int) bool {
	if l.Players[playerIndex].Status.Ready {
		return true
	}
	return false
}

func (l *lobby) UnReadyAllPlayers() {
	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr != nil {
			l.Players[i].Status.Ready = false
			l.Players[i].Status.Dead = false
		}
	}
}

func (l *lobby) SpawnPlayers() {
	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr != nil && !l.Players[i].Status.Spawned {
			l.SpawnPlayer(i, l.Players[i].Status.Position.Position, l.Players[i].Status.Position.Rotation)
		}
	}
}

func (l *lobby) SpawnPlayer(playerIndex int, position, rotation vector2) {
	l.Lock()
	defer l.Unlock()

	flag := byte(0) //0 (default) = revive player for new map, 1 = forced die for spawned player
	if !l.IsInLobby() && l.GetPlayersInLobby(playerIndex) > 1 {
		flag = byte(1)
	}

	packetClientSpawned := newPacket(packetTypeClientSpawned, 0, 0)
	packetClientSpawned.Grow(26)
	packetClientSpawned.WriteByteNext(byte(playerIndex))
	packetClientSpawned.WriteF32LENext([]float32{
		position.X, position.Y, 0,
		rotation.X, rotation.Y, 0,
	})
	packetClientSpawned.WriteByteNext(flag)

	l.Players[playerIndex].Status.Spawned = true
	l.Players[playerIndex].UpdateChannel = playerIndex*2 + 2
	l.Players[playerIndex].EventChannel = l.Players[playerIndex].UpdateChannel + 1

	log.Info("Spawned player ", playerIndex, " at position ", position, " with rotation ", rotation, " using flag ", flag)
	l.Broadcast(packetClientSpawned, nil) //Tell all players that the new client has spawned

	if l.MapIndex > -1 {
		log.Info("Telling player ", playerIndex, " to start match")
		l.SendTo(newPacket(packetTypeStartMatch, 1, 0), l.Players[playerIndex].Addr)
	}
}

func (l *lobby) AddPlayer(addr *net.UDPAddr) (playerIndex int, err error) {
	l.Lock()
	defer l.Unlock()

	for _, pl := range l.Players {
		if pl.Addr == nil || pl.Addr.String() == addr.String() {
			break
		}
		playerIndex++
	}

	if playerIndex >= l.MaxPlayers {
		return -1, errors.New("lobby has reached max capacity")
	}

	l.Players[playerIndex] = player{
		Addr: addr,
	}

	return playerIndex, nil
}

func (l *lobby) GetPlayer(addr *net.UDPAddr) *player {
	for _, pl := range l.Players {
		if pl.Addr.String() == addr.String() {
			return &pl
		}
	}
	return nil
}

func (l *lobby) GetPlayerIndex(addr *net.UDPAddr) int {
	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr.String() == addr.String() {
			return i
		}
	}
	return -1
}

func (l *lobby) GetHostIndex() int {
	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr != nil {
			return i
		}
	}
	return -1
}

func (l *lobby) KickPlayerIndex(playerIndex int) error {
	if playerIndex >= l.MaxPlayers {
		return errors.New("cannot kick out of bounds player index")
	}

	if l.Players[playerIndex].Addr == nil {
		return errors.New("tried to kick player that isn't in lobby")
	}

	l.SendTo(newPacket(packetTypeKickPlayer, 1, 0), l.Players[playerIndex].Addr)
	l.Players[playerIndex] = player{}

	return nil
}

func (l *lobby) KickPlayerSteamID(steamID uint64) error {
	playersTried := 0
	for i := 0; i < len(l.Players); i++ {
		pl := l.Players[i]
		if pl.SteamID != steamID {
			playersTried++
			continue
		}
		l.Players[i] = player{}
	}
	if playersTried == l.GetPlayersInLobby(-1) {
		return errors.New("tried to kick player that isn't in lobby")
	}

	return nil
}

func (l *lobby) SetPlayerSteamID(addr *net.UDPAddr, steamID uint64) {
	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr.String() == addr.String() {
			l.Players[i].SteamID = steamID
		}
	}
}
