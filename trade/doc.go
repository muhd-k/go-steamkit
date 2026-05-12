// Package trade provides Steam trade offer management over Steam's Web APIs.
//
// The package is built around [Client], which uses an authenticated
// [auth.SteamSession] for Steam Community requests. A Steam Web API key is
// optional — if omitted, the client's access token is used for IEconService
// calls instead.
//
// Basic usage without an API key:
//
//	sess, _ := auth.NewSession(auth.WithRefreshToken(savedRefreshToken))
//	_, _ = sess.ObtainCookies(ctx)
//
//	client, _ := trade.NewClient(sess, "")
//	offers, err := client.GetActiveOffers(ctx, true)
//
// With an API key (can improve pagination limits for historical offers):
//
//	client, _ := trade.NewClient(sess, apiKey)
//
// Sending an offer:
//
//	offerID, err := client.Send(ctx, trade.SendOptions{
//	    PartnerSteamID: partnerID,
//	    AccessToken:    tradeToken,
//	    MyItems: []trade.Item{
//	        {AppID: 730, ContextID: 2, AssetID: 123, Amount: 1},
//	    },
//	})
package trade
