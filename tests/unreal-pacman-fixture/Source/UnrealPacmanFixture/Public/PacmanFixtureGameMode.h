#pragma once

#include "GameFramework/GameModeBase.h"
#include "PacmanFixtureGameMode.generated.h"

UCLASS()
class UNREALPACMANFIXTURE_API APacmanFixtureGameMode final : public AGameModeBase
{
    GENERATED_BODY()

protected:
    virtual void StartPlay() override;
};
