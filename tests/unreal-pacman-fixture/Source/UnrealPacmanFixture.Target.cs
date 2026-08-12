using UnrealBuildTool;

public class UnrealPacmanFixtureTarget : TargetRules
{
    public UnrealPacmanFixtureTarget(TargetInfo Target) : base(Target)
    {
        Type = TargetType.Game;
        DefaultBuildSettings = BuildSettingsVersion.Latest;
        IncludeOrderVersion = EngineIncludeOrderVersion.Unreal5_8;
        ExtraModuleNames.AddRange(new[] { "UnrealPacmanFixture", "JangolovaPacman" });
    }
}
