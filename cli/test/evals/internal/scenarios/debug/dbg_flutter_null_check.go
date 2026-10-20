package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/flutter-null-check", dbgSeedFlutterNullCheck) }

// dbgSeedFlutterNullCheck: the product page's hero carousel null-checks
// product.media, which the catalog omits for listings without photos. Such
// listings started appearing about a day ago (marketplace sellers), so the
// crash hits both app versions in the field and no app release lines up
// with it. Distractor: a steady trickle of host-lookup failures (offline
// phones) from the same API client.
func dbgSeedFlutterNullCheck(s *seed.Scenario) error {
	p := s.Project("shop-flutter", seed.Mobile, "flutter")
	const fw = "package:flutter/src/widgets/framework.dart"
	// The traceway package reports each frame as "function\n    location".
	nullCheck := "_TypeError: Null check operator used on a null value\n" +
		"ProductHeroCarousel.build\n    package:shop_app/features/product/widgets/product_hero_carousel.dart:24:32\n" +
		"StatelessElement.build\n    " + fw + ":5687:49\n" +
		"ComponentElement.performRebuild\n    " + fw + ":5617:15\n" +
		"Element.rebuild\n    " + fw + ":5333:7\n" +
		"ComponentElement._firstBuild\n    " + fw + ":5599:5\n" +
		"ComponentElement.mount\n    " + fw + ":5593:5\n" +
		"Element.inflateWidget\n    " + fw + ":4468:20\n" +
		"Element.updateChild\n    " + fw + ":3963:18\n" +
		"SliverMultiBoxAdaptorElement.updateChild\n    package:flutter/src/widgets/sliver.dart:1022:37\n" +
		"SliverMultiBoxAdaptorElement.createChild.<anonymous closure>\n    package:flutter/src/widgets/sliver.dart:892:20\n" +
		"BuildOwner.buildScope\n    " + fw + ":3045:19\n" +
		"SliverMultiBoxAdaptorElement.createChild\n    package:flutter/src/widgets/sliver.dart:884:12\n" +
		"RenderSliverMultiBoxAdaptor._createOrObtainChild.<anonymous closure>\n    package:flutter/src/rendering/sliver_multi_box_adaptor.dart:376:23\n" +
		"RenderObject.invokeLayoutCallback.<anonymous closure>\n    package:flutter/src/rendering/object.dart:2876:17\n" +
		"PipelineOwner._enableMutationsToDirtySubtrees\n    package:flutter/src/rendering/object.dart:1170:15\n" +
		"RenderObject.invokeLayoutCallback\n    package:flutter/src/rendering/object.dart:2875:14\n" +
		"RenderSliverMultiBoxAdaptor._createOrObtainChild\n    package:flutter/src/rendering/sliver_multi_box_adaptor.dart:365:5\n" +
		"RenderSliverMultiBoxAdaptor.addInitialChild\n    package:flutter/src/rendering/sliver_multi_box_adaptor.dart:461:5\n" +
		"RenderSliverList.performLayout\n    package:flutter/src/rendering/sliver_list.dart:58:12\n" +
		"RenderObject.layout\n    package:flutter/src/rendering/object.dart:2706:7\n" +
		"PipelineOwner.flushLayout\n    package:flutter/src/rendering/object.dart:1144:18\n" +
		"RendererBinding.drawFrame\n    package:flutter/src/rendering/binding.dart:622:23\n" +
		"WidgetsBinding.drawFrame\n    package:flutter/src/widgets/binding.dart:1241:13\n" +
		"RendererBinding._handlePersistentFrameCallback\n    package:flutter/src/rendering/binding.dart:482:5\n" +
		"SchedulerBinding._invokeFrameCallback\n    package:flutter/src/scheduler/binding.dart:1442:15\n" +
		"SchedulerBinding.handleDrawFrame\n    package:flutter/src/scheduler/binding.dart:1355:9"
	devices := []string{"iPhone15,2", "iPhone16,1", "iPhone14,7", "Pixel 8", "SM-S921B", "SM-A546B", "Pixel 7a", "2312DRA50G"}

	for _, at := range dbgTimes(s, s.Ago(21*time.Hour), s.Ago(5*time.Minute), 72, true) {
		v := "4.3.0"
		if s.RNG.Float64() < 0.22 {
			v = "4.2.1"
		}
		c := seed.Crash{At: at, Stack: nullCheck, AppVersion: v, Device: dbgPick(s, devices)}
		if at.After(s.Ago(10 * time.Hour)) {
			id := 40000 + s.RNG.IntN(50000)
			c.Actions = []seed.Action{
				{Type: "navigation", At: at.Add(-time.Duration(900+s.RNG.IntN(1500)) * time.Millisecond), From: "/", To: fmt.Sprintf("/products/%d", id)},
				{Type: "network", At: at.Add(-time.Duration(500+s.RNG.IntN(600)) * time.Millisecond), Method: "GET", URL: fmt.Sprintf("https://api.shop.example/v1/products/%d", id), DurationMs: float64(90 + s.RNG.IntN(250)), Status: 200, RespBytes: 780 + s.RNG.IntN(300)},
			}
		}
		p.Crash(c)
	}
	for _, at := range dbgTimes(s, s.Ago(7*24*time.Hour), s.Ago(50*time.Minute), 22, false) {
		id := 40000 + s.RNG.IntN(50000)
		v := "4.3.0"
		if s.RNG.Float64() < 0.3 {
			v = "4.2.1"
		}
		stack := fmt.Sprintf("_ClientSocketException: ClientException with SocketException: Failed host lookup: 'api.shop.example' (OS Error: No address associated with hostname, errno = 7), uri=https://api.shop.example/v1/products/%d\n", id) +
			"IOClient.send\n    package:http/src/io_client.dart:227:7\n" +
			"BaseClient._sendUnstreamed\n    package:http/src/base_client.dart:93:32\n" +
			"ApiClient.product\n    package:shop_app/core/api_client.dart:14:17\n" +
			"_ProductPageState._load\n    package:shop_app/features/product/product_page.dart:22:14"
		p.Crash(seed.Crash{At: at, Stack: stack, AppVersion: v, Device: dbgPick(s, devices)})
	}

	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "ProductHeroCarousel.build")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "Failed host lookup")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		return nil
	})
	return nil
}
