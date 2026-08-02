#include "PacmanFixtureActor.h"

#include "Components/SceneComponent.h"
#include "PacmanProtocol.h"
#include "PacmanRegistryComponent.h"

APacmanFixtureActor::APacmanFixtureActor()
{
    PrimaryActorTick.bCanEverTick = false;
    RootComponent = CreateDefaultSubobject<USceneComponent>(TEXT("FixtureRoot"));
    PacmanRegistry = CreateDefaultSubobject<UPacmanRegistryComponent>(TEXT("PacmanRegistry"));

    FPacmanRegistration Registration;
    Registration.StableId = TEXT("object:fixture");
    Registration.Kind = EPacmanResourceKind::Object;
    Registration.Label = TEXT("Pacman fixture actor");
    Registration.Target = this;
    Registration.Actions = { TEXT("resource.describe"), TEXT("object.visibility.set") };
    PacmanRegistry->Registrations.Add(Registration);
}
