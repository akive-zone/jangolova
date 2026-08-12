#pragma once

#include "CoreMinimal.h"
#include "IPacmanTransportHost.h"
#include "UObject/WeakObjectPtr.h"

class FPacmanRequestRouter;

// Authenticates and routes an already accepted caller-owned WebSocket. A
// platform/server binding calls AcceptConnection after performing its HTTP
// upgrade; this class owns no listen socket or Unreal process lifecycle.
class JANGOLOVAPACMAN_API FPacmanWebSocketHost final : public IPacmanTransportHost
{
public:
    explicit FPacmanWebSocketHost(FString InBearerToken);
    virtual ~FPacmanWebSocketHost() override;

    virtual void StartHost(TWeakObjectPtr<UPacmanRegistryComponent> Registry) override;
    virtual void StopHost() override;

    bool AcceptConnection(const TSharedRef<IPacmanWebSocketConnection>& Connection);

private:
    static bool ConstantTimeEquals(const FString& Left, const FString& Right);
    static bool IsAuthMessage(const FString& Message, const FString& Token);

    FString BearerToken;
    TWeakObjectPtr<UPacmanRegistryComponent> Registry;
    TSharedPtr<FPacmanRequestRouter> Router;
    TSharedPtr<IPacmanWebSocketConnection> ActiveConnection;
    FCriticalSection Mutex;
    bool Started = false;
};
