// Package inventory provides Steam inventory fetching from the Steam Community website.
//
// The package is built around [Client], which uses an authenticated
// [auth.SteamSession] for Steam Community requests.
//
// Basic usage:
//
//	sess, _ := auth.NewSession(auth.WithRefreshToken(savedRefreshToken))
//	_, _ = sess.ObtainCookies(ctx)
//
//	client, _ := inventory.NewClient(sess)
//	items, err := client.GetAllMyInventory(ctx, 730, 2, nil)
//
// Fetching with asset properties (CS2 stickers, wear, float, etc.):
//
//	items, err := client.GetAllMyInventory(ctx, 730, 2, &inventory.GetInventoryOptions{
//	    IncludeProperties: true,
//	})
package inventory
