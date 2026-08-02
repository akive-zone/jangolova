#include "PacmanFixtureGameMode.h"

#include "Engine/World.h"
#include "PacmanFixtureActor.h"

void APacmanFixtureGameMode::StartPlay()
{
    Super::StartPlay();
    if (GetWorld() == nullptr) return;
    FActorSpawnParameters SpawnParameters;
    SpawnParameters.Name = TEXT("PacmanFixtureActor");
    GetWorld()->SpawnActor<APacmanFixtureActor>(APacmanFixtureActor::StaticClass(), FTransform::Identity, SpawnParameters);
}
