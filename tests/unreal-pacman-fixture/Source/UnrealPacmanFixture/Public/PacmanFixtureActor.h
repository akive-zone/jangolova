#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Actor.h"
#include "PacmanFixtureActor.generated.h"

class UPacmanRegistryComponent;

UCLASS()
class UNREALPACMANFIXTURE_API APacmanFixtureActor final : public AActor
{
    GENERATED_BODY()

public:
    APacmanFixtureActor();

    UPROPERTY(VisibleAnywhere, BlueprintReadOnly, Category = "Pacman")
    TObjectPtr<UPacmanRegistryComponent> PacmanRegistry;
};
