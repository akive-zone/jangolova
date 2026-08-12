#include "PacmanWebSocketHost.h"

#include "Misc/ScopeLock.h"
#include "Dom/JsonObject.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"
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
    const bool HeaderAuthenticated = ConstantTimeEquals(Connection->AuthorizationHeader(), FString::Printf(TEXT("Bearer %s"), *BearerToken));
    if (!Connection->AuthorizationHeader().IsEmpty() && !HeaderAuthenticated)
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
    TSharedRef<TAtomic<bool>> Authenticated = MakeShared<TAtomic<bool>>(HeaderAuthenticated);
    const FString ExpectedToken = BearerToken;
    Connection->SetTextHandler([CurrentRouter, Connection, Authenticated, ExpectedToken](const FString& Message)
    {
        if (!CurrentRouter.IsValid()) return;
        if (!Authenticated->Load())
        {
            if (!IsAuthMessage(Message, ExpectedToken))
            {
                Connection->Close();
                return;
            }
            Authenticated->Store(true);
            Connection->SendText(TEXT("{\"type\":\"pacman.authenticated\"}"));
            return;
        }
        CurrentRouter->HandleText(Message, [Connection](const FString& Reply)
        {
            Connection->SendText(Reply);
        }, CurrentRouter);
    });
    return true;
}

bool FPacmanWebSocketHost::IsAuthMessage(const FString& Message, const FString& Token)
{
    const TSharedRef<TJsonReader<TCHAR>> Reader = TJsonReaderFactory<TCHAR>::Create(Message);
    TSharedPtr<FJsonObject> Object;
    if (!FJsonSerializer::Deserialize(Reader, Object) || !Object.IsValid()) return false;
    FString Type;
    FString Candidate;
    return Object->TryGetStringField(TEXT("type"), Type)
        && Object->TryGetStringField(TEXT("token"), Candidate)
        && Type == TEXT("auth")
        && ConstantTimeEquals(Candidate, Token);
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
