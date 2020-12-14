package main

import (
	"strconv"
	"strings"
)

func onPlayerUpdate(p *packet, l *lobby) {
	//Position package
	position := vector2{
		X: float32(p.ReadI16LENext(1)[0]),
		Y: float32(p.ReadI16LENext(1)[0]),
	}
	rotation := vector2{
		X: float32(p.ReadByteNext()),
		Y: float32(p.ReadByteNext()),
	}
	yValue := int(p.ReadByteNext())
	movement := movementType(p.ReadByteNext())

	//Weapon package
	fight := fightState(p.ReadByteNext())
	projectileCount := int(p.ReadU16LENext(1)[0])
	projectiles := make([]projectile, projectileCount)
	if projectileCount > 0 {
		for i := 0; i < projectileCount; i++ {
			projectiles[i].ShootPosition.X = float32(p.ReadI16LENext(1)[0])
			projectiles[i].ShootPosition.Y = float32(p.ReadI16LENext(1)[0])
			projectiles[i].Shoot.X = float32(p.ReadByteNext())
			projectiles[i].Shoot.Y = float32(p.ReadByteNext())
			projectiles[i].SyncIndex = p.ReadU16LENext(1)[0]
		}
	}
	weapon := weaponType(p.ReadByteNext())

	netPosition := networkPosition{
		Position:     position,
		Rotation:     rotation,
		YValue:       yValue,
		MovementType: movement,
	}
	netWeapon := networkWeapon{
		FightState:  fight,
		Projectiles: projectiles,
		WeaponType:  weapon,
	}

	for i := 0; i < len(l.Players); i++ {
		if l.Players[i].Addr == p.Src {
			if !l.Players[i].Status.Moved {
				log.Debug("Player ", i, " has moved!")
				l.Players[i].Status.Moved = true
			}
			l.Players[i].Status.Position = netPosition
			l.Players[i].Status.Weapon = netWeapon
			break
		}
	}

	log.Trace("New client state for ", p.Src, ": Position(", position, ") Rotation(", rotation, ") YValue:", yValue, " Movement:", movement, " Fight:", fight, " Weapon:", weapon, " Projectiles:", projectiles)

	l.Broadcast(p, p.Src)
}

func onPlayerTookDamage(p *packet, l *lobby) {
	playerIndex := l.GetPlayerIndex(p.Src)

	if l.Players[playerIndex].Status.Dead {
		log.Warn("Player ", playerIndex, " took damage despite being dead! Tossing...")
		return
	}
	if !l.IsPlayerReady(playerIndex) && l.MapIndex > -1 { //Make sure player is ready if not in lobby map
		log.Warn("Player ", playerIndex, " took damage despite not being ready! Tossing...")
		return
	}

	attackerIndex := int(p.ReadByteNext())
	if attackerIndex == playerIndex {
		log.Warn("Player ", playerIndex, " tried damaging themselves, just a spawn glitch! Should toss...")
		//return
	}

	damage := p.ReadF32LENext(1)[0]
	killingBlow := false
	if damage == 666.666 {
		killingBlow = true
	}

	typeDamage := byte(damageTypeOther)

	playParticles := p.ReadByteNext()
	particleDirection := vector3{}
	if playParticles == 1 {
		particleDirection.X = p.ReadF32LENext(1)[0]
		particleDirection.Y = p.ReadF32LENext(1)[0]

		if p.ByteCapacity() > 14 {
			typeDamage = p.ReadByteNext()
		}
	} else if p.ByteCapacity() > 6 {
		typeDamage = p.ReadByteNext()
	}

	if damageType(typeDamage) == damageTypePunch && playerIndex != attackerIndex {
		l.Players[attackerIndex].Stats.PunchesLanded++
	}

	if killingBlow {
		log.Info("Player ", playerIndex, " took a killing blow from player ", attackerIndex, " of type ", typeDamage)
		l.Players[playerIndex].Status.Health = 0
		l.Players[playerIndex].Status.Dead = true
		l.Players[playerIndex].Stats.Deaths++

		l.Players[attackerIndex].Stats.Kills++

		if l.GetPlayersInLobby(playerIndex) > 0 {
			l.CheckWinner(attackerIndex)
		}
	} else {
		log.Info("Player ", playerIndex, " took ", damage, " damage from player ", attackerIndex, " of type ", typeDamage)
		l.Players[playerIndex].Status.Health -= damage
	}

	l.Broadcast(p, p.Src)
}

func onPlayerTalked(p *packet, l *lobby) {
	l.Broadcast(p, p.Src)

	msg := string(p.Bytes())
	if l.GetHostIndex() == l.GetPlayerIndex(p.Src) && string(msg[0]) == "/" {
		respMsg := ""

		cmd := strings.Split(string(msg[1:]), " ")
		switch cmd[0] {
		case "echo":
			if len(cmd) < 2 {
				respMsg = "Must specify message to echo!"
				break
			}
			respMsg = strings.Join(cmd[1:], " ")
		case "map":
			if len(cmd) < 2 {
				respMsg = "Must specify map index!"
				break
			}
			mapIndex, err := strconv.Atoi(cmd[1])
			if err != nil || mapIndex >= len(l.Maps) || mapIndex < 0 {
				respMsg = "Invalid map index! 0-" + strconv.Itoa(len(l.Maps)-1)
				break
			}
			l.ChangeMap(mapIndex)
			respMsg = "Map changed!"
		case "kick":
			if len(cmd) < 2 {
				respMsg = "Must specify player to kick!"
				break
			}
			switch cmd[1] {
			case "1", "yellow", "y":
				if l.GetPlayerIndex(p.Src) == 0 {
					respMsg = "Can't kick yourself!"
					break
				}
				l.KickPlayerIndex(0)
			case "2", "blue", "b":
				if l.GetPlayerIndex(p.Src) == 1 {
					respMsg = "Can't kick yourself!"
					break
				}
				l.KickPlayerIndex(1)
			case "3", "red", "r":
				if l.GetPlayerIndex(p.Src) == 2 {
					respMsg = "Can't kick yourself!"
					break
				}
				l.KickPlayerIndex(2)
			case "4", "green", "g":
				if l.GetPlayerIndex(p.Src) == 3 {
					respMsg = "Can't kick yourself!"
					break
				}
				l.KickPlayerIndex(3)
			default:
				respMsg = "Unknown player!"
				break
			}
			respMsg = "Kicked player: " + cmd[1]
		default:
			respMsg = "Unknown command!"
		}

		resp := newPacket(packetTypePlayerTalked, l.Players[l.GetHostIndex()].EventChannel, l.Players[l.GetHostIndex()].SteamID)
		resp.Grow(int64(len(respMsg)))
		resp.WriteBytesNext([]byte(respMsg))
		l.SendTo(resp, p.Src)
	}
}

func onPlayerFallOut(p *packet, l *lobby) {
	playerIndex := int(p.ReadByteNext())

	log.Info("Player ", playerIndex, " fell out of the map")
	l.Players[playerIndex].Status.Health = 0
	l.Players[playerIndex].Status.Dead = true
	l.Players[playerIndex].Stats.Deaths++

	l.CheckWinner(playerIndex)

	l.Broadcast(p, p.Src)
}
