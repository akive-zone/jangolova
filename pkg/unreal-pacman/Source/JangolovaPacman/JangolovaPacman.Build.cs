using UnrealBuildTool;

public class JangolovaPacman : ModuleRules
{
    public JangolovaPacman(ReadOnlyTargetRules Target) : base(Target)
    {
        PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;
        PublicDependencyModuleNames.AddRange(new[] { "Core", "CoreUObject", "Engine", "Json", "WebSocketServer" });
    }
}
