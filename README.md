# Stick Fight: The Dedicated Server

This repository contains the code for a dedicated server for Stick Fight: The Game. The server is written in Go.

## About

Stick Fight: The Game is a popular online fighting game. This project aims to provide a dedicated server to enhance the gaming experience. The server is being developed in spare time, so updates may be sporadic.

## Dependencies

- `Steamcmd` : to download the map data
- `go` : to build/run the server
- `dotnet-sdk-3.0` : for running the map decoder
- `libicu-devel` : ICU package for Globalization

## Repository Structure

Here's a brief overview of the main files and directories in this repository:

- `clients.go`: Contains the code related to the clients.
- `codes.go`: Contains various codes used in the server.
- `combat.go`: Handles the combat logic.
- `gamemodes.go`: Defines the different game modes.
- `gm_duel.go`, `gm_gungame.go`, `gm_stock.go`, `gm_tournament.go`: These files contain the logic for the respective game modes.
- `levels.go`: Handles the game levels.
- `lobbies.go`: Manages the game lobbies.
- `main.go`: The entry point of the server.
- `objects.go`: Defines the game objects.
- `packets.go`: Handles the network packets.
- `players.go`: Manages the players.
- `server.go`: Contains the main server logic.
- `steam.go`: Handles the Steam integration.
- `types.go`: Defines various types used in the server.

## Contributing

Contributions are welcome! If you know enough C# (and Go!) to help, feel free to fork the repository and submit a pull request.
