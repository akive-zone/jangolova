#include "Misc/AutomationTest.h"

#include "PacmanFixtureActor.h"
#include "PacmanRegistryComponent.h"

#if WITH_DEV_AUTOMATION_TESTS
IMPLEMENT_SIMPLE_AUTOMATION_TEST(
    FPacmanFixtureRegistrationTest,
    "Jangolova.Pacman.Fixture.ExplicitRegistration",
    EAutomationTestFlags::EditorContext | EAutomationTestFlags::EngineFilter)

bool FPacmanFixtureRegistrationTest::RunTest(const FString& Parameters)
{
    const APacmanFixtureActor* Defaults = GetDefault<APacmanFixtureActor>();
    TestNotNull(TEXT("Fixture actor defaults exist"), Defaults);
    if (Defaults == nullptr || Defaults->PacmanRegistry == nullptr) return false;
    TestEqual(TEXT("Exactly one resource is registered"), Defaults->PacmanRegistry->Registrations.Num(), 1);
    if (Defaults->PacmanRegistry->Registrations.Num() != 1) return false;
    const FPacmanRegistration& Registration = Defaults->PacmanRegistry->Registrations[0];
    TestEqual(TEXT("Stable fixture ID"), Registration.StableId, FString(TEXT("object:fixture")));
    TestTrue(TEXT("Fixture target is explicitly assigned"), Registration.Target != nullptr);
    TestTrue(TEXT("Visibility action is explicitly allowlisted"), Registration.Actions.Contains(TEXT("object.visibility.set")));
    return true;
}
#endif
