<?php
/*
 * Status > Network Monitor page for the pfSense web GUI.
 *
 * Installed to /usr/local/www by install.sh. The dashboard is loaded through
 * netmon_proxy.php, so it is protected by the pfSense login and served over the
 * GUI's HTTPS, and the monitor itself only has to listen on 127.0.0.1.
 */

require_once("guiconfig.inc");

$pgtitle = array(gettext("Status"), "Network Monitor");
include("head.inc");
?>

<div class="panel panel-default">
	<div class="panel-body" style="padding:0">
		<iframe id="netmon-frame" src="/netmon_proxy.php?p=" title="Network Monitor"
			style="display:block;width:100%;height:80vh;border:0"></iframe>
	</div>
</div>

<script type="text/javascript">
//<![CDATA[
// The dashboard reports its height so the frame grows instead of scrolling twice.
window.addEventListener("message", function (e) {
	if (e.origin !== window.location.origin || !e.data || !e.data.netmonHeight) {
		return;
	}
	document.getElementById("netmon-frame").style.height = Math.max(400, e.data.netmonHeight) + "px";
});
//]]>
</script>

<?php include("foot.inc"); ?>
