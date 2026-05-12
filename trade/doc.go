// Package trade provides Steam trade offer management over Steam's Web APIs.
//
// The package is built around [Client], which uses an authenticated
// [auth.SteamSession] for Steam Community requests and a Steam Web API key for
// IEconService trade-offer reads and actions.
//
// Basic usage:
//
//	sess, _ := auth.NewSession(auth.WithRefreshToken(savedRefreshToken))
//	_, _ = sess.ObtainCookies(ctx)
//
//	client, _ := trade.NewClient(sess, apiKey)
//	offers, err := client.GetActiveOffers(ctx, true)
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
