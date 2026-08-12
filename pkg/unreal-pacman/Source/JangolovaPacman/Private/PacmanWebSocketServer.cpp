#include "PacmanWebSocketServer.h"

#include "IWebSocketClientConnection.h"
#include "IWebSocketServer.h"
#include "PacmanRegistryComponent.h"
#include "PacmanWebSocketHost.h"
#include "WebSocketServerModule.h"

namespace
{
}

FPacmanUnrealWebSocketConnection::FPacmanUnrealWebSocketConnection(const TSharedRef<IWebSocketClientConnection>& InConnection)
    : Connection(InConnection)
{
}

FString FPacmanUnrealWebSocketConnection::AuthorizationHeader() const { return FString(); }
void FPacmanUnrealWebSocketConnection::SetTextHandler(FTextHandler InHandler) { Handler = MoveTemp(InHandler); }
void FPacmanUnrealWebSocketConnection::SendText(const FString& Message) { Connection->SendText(Message); }
void FPacmanUnrealWebSocketConnection::Close() { Connection->Close(1000, TEXT("Pacman transport closed")); }
void FPacmanUnrealWebSocketConnection::Deliver(const FString& Message) { if (Handler) Handler(Message); }

FPacmanWebSocketServer::FPacmanWebSocketServer(FString InBearerToken)
    : BearerToken(MoveTemp(InBearerToken))
{
}

FPacmanWebSocketServer::~FPacmanWebSocketServer()
{
    Stop();
}

bool FPacmanWebSocketServer::Start(uint16 Port, TWeakObjectPtr<UPacmanRegistryComponent> Registry)
{
    if (Listening || Port == 0 || BearerToken.IsEmpty() || !Registry.IsValid()) return false;
    Host = MakeUnique<FPacmanWebSocketHost>(BearerToken);
    Host->StartHost(Registry);
    Server = FWebSocketServerModule::Get().GetWebSocketServer(Port);
    if (!Server.IsValid()) return false;

    Server->OnConnected([this](TSharedRef<IWebSocketClientConnection> Connection)
    {
        const TSharedPtr<FPacmanUnrealWebSocketConnection> Wrapped = MakeShared<FPacmanUnrealWebSocketConnection>(Connection);
        Connections.Add(Connection.Get(), Wrapped);
        if (!Host->AcceptConnection(Wrapped.ToSharedRef()))
        {
            Connections.Remove(Connection.Get());
            return;
        }
    });
    Server->OnMessage([this](TSharedRef<IWebSocketClientConnection> Connection, const FString& Message)
    {
        if (const TSharedPtr<FPacmanUnrealWebSocketConnection>* Wrapped = Connections.Find(Connection.Get()))
        {
            (*Wrapped)->Deliver(Message);
        }
    });
    Server->OnDisconnected([this](TSharedRef<IWebSocketClientConnection> Connection)
    {
        Connections.Remove(Connection.Get());
    });
    FWebSocketServerModule::Get().StartAllServers();
    Listening = Server->IsListening();
    return Listening;
}

void FPacmanWebSocketServer::Stop()
{
    if (Server.IsValid() && Listening) Server->StopListening();
    Listening = false;
    if (Host) Host->StopHost();
    Connections.Empty();
    Host.Reset();
    Server.Reset();
}

bool FPacmanWebSocketServer::IsListening() const
{
    return Listening && Server.IsValid() && Server->IsListening();
}
