using UnrealBuildTool;

public class UnrealPacmanFixtureTarget : TargetRules
{
    public UnrealPacmanFixtureTarget(TargetInfo Target) : base(Target)
    {
        Type = TargetType.Game;
        DefaultBuildSettings = BuildSettingsVersion.V5;
        IncludeOrderVersion = EngineIncludeOrderVersion.Unreal5_3;
        ExtraModuleNames.Add("UnrealPacmanFixture");
    }
}
