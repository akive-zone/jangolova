using UnrealBuildTool;

public class UnrealPacmanFixtureEditorTarget : TargetRules
{
    public UnrealPacmanFixtureEditorTarget(TargetInfo Target) : base(Target)
    {
        Type = TargetType.Editor;
        DefaultBuildSettings = BuildSettingsVersion.V5;
        IncludeOrderVersion = EngineIncludeOrderVersion.Unreal5_3;
        ExtraModuleNames.Add("UnrealPacmanFixture");
    }
}
