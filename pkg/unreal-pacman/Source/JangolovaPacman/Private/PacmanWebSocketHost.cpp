#include "PacmanWebSocketHost.h"

#include "Misc/ScopeLock.h"
#include "PacmanRequestRouter.h"

FPacmanWebSocketHost::FPacmanWebSocketHost(FString InBearerToken)
    : BearerToken(MoveTemp(InBearerToken))
{
}

FPacmanWebSocketHost::~FPacmanWebSocketHost()
{
    StopHost();
}

void FPacmanWebSocketHost::StartHost(TWeakObjectPtr<UPacmanRegistryComponent> InRegistry)
{
    FScopeLock Lock(&Mutex);
    Registry = InRegistry;
    Router = MakeShared<FPacmanRequestRouter>(Registry);
    Started = true;
}

void FPacmanWebSocketHost::StopHost()
{
    TSharedPtr<IPacmanWebSocketConnection> Connection;
    TSharedPtr<FPacmanRequestRouter> CurrentRouter;
    {
        FScopeLock Lock(&Mutex);
        if (!Started && !ActiveConnection.IsValid()) return;
        Started = false;
        CurrentRouter = Router;
        Router.Reset();
        Connection = ActiveConnection;
        ActiveConnection.Reset();
        Registry.Reset();
    }
    if (CurrentRouter.IsValid()) CurrentRouter->Stop();
    if (Connection.IsValid()) Connection->Close();
}

bool FPacmanWebSocketHost::AcceptConnection(const TSharedRef<IPacmanWebSocketConnection>& Connection)
{
    if (BearerToken.IsEmpty())
    {
        Connection->Close();
        return false;
    }
    if (!ConstantTimeEquals(Connection->AuthorizationHeader(), FString::Printf(TEXT("Bearer %s"), *BearerToken)))
    {
        Connection->Close();
        return false;
    }

    TSharedPtr<FPacmanRequestRouter> CurrentRouter;
    TSharedPtr<IPacmanWebSocketConnection> Previous;
    {
        FScopeLock Lock(&Mutex);
        if (!Started || !Router.IsValid())
        {
            Connection->Close();
            return false;
        }
        Previous = ActiveConnection;
        ActiveConnection = Connection;
        CurrentRouter = Router;
    }
    if (Previous.IsValid()) Previous->Close();
    Connection->SetTextHandler([CurrentRouter](const FString& Message)
    {
        if (!CurrentRouter.IsValid()) return;
        CurrentRouter->HandleText(Message, [Connection](const FString& Reply)
        {
            Connection->SendText(Reply);
        });
    });
    return true;
}

bool FPacmanWebSocketHost::ConstantTimeEquals(const FString& Left, const FString& Right)
{
    if (Left.Len() != Right.Len()) return false;
    uint32 Difference = 0;
    for (int32 Index = 0; Index < Left.Len(); ++Index)
    {
        Difference |= static_cast<uint32>(Left[Index]) ^ static_cast<uint32>(Right[Index]);
    }
    return Difference == 0;
}
