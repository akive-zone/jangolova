#include "PacmanFixtureGameMode.h"

#include "Engine/World.h"
#include "Misc/CommandLine.h"
#include "Misc/Parse.h"
#include "Misc/PlatformMisc.h"
#include "PacmanFixtureActor.h"
#include "PacmanWebSocketServer.h"

void APacmanFixtureGameMode::StartPlay()
{
    Super::StartPlay();
    if (GetWorld() == nullptr) return;
    FActorSpawnParameters SpawnParameters;
    SpawnParameters.Name = TEXT("PacmanFixtureActor");
    APacmanFixtureActor* Fixture = GetWorld()->SpawnActor<APacmanFixtureActor>(APacmanFixtureActor::StaticClass(), FTransform::Identity, SpawnParameters);
    const FString Token = FPlatformMisc::GetEnvironmentVariable(TEXT("JANGOLOVA_PACMAN_TOKEN"));
    int32 Port = 8090;
    FParse::Value(FCommandLine::Get(), TEXT("PacmanPort="), Port);
    if (Fixture != nullptr && Fixture->PacmanRegistry != nullptr && !Token.IsEmpty())
    {
        PacmanServer = MakeUnique<FPacmanWebSocketServer>(Token);
        if (!PacmanServer->Start(static_cast<uint16>(Port), Fixture->PacmanRegistry))
        {
            UE_LOG(LogTemp, Error, TEXT("Unable to start Jangolova Pacman WebSocket server on port %d"), Port);
        }
    }
}

void APacmanFixtureGameMode::EndPlay(const EEndPlayReason::Type EndPlayReason)
{
    if (PacmanServer) PacmanServer->Stop();
    PacmanServer.Reset();
    Super::EndPlay(EndPlayReason);
}
