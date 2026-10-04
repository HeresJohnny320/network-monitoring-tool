<?php
/*
 * Adds or removes the Status > Network Monitor menu entry in pfSense.
 * Run by install.sh:  php -f netmon_menu.php install|uninstall
 */

require_once("config.inc");
require_once("util.inc");

$action = isset($argv[1]) ? $argv[1] : 'install';
$url = '/status_netmon.php';

if (function_exists('config_get_path')) {
	$menus = config_get_path('installedpackages/menu', array());
} else {
	global $config;
	$menus = isset($config['installedpackages']['menu']) ? $config['installedpackages']['menu'] : array();
}
if (!is_array($menus)) {
	$menus = array();
}

// Drop any existing entry first so re-running the installer doesn't duplicate it.
$kept = array();
foreach ($menus as $menu) {
	if (!is_array($menu) || !isset($menu['url']) || $menu['url'] !== $url) {
		$kept[] = $menu;
	}
}
if ($action === 'install') {
	$kept[] = array(
		'name' => 'Network Monitor',
		'tooltiptext' => 'ISP speed, ping and traceroute history',
		'section' => 'Status',
		'url' => $url,
	);
}

if (function_exists('config_set_path')) {
	if (count($kept)) {
		config_set_path('installedpackages/menu', $kept);
	} else {
		config_del_path('installedpackages/menu');
	}
} else {
	if (count($kept)) {
		$config['installedpackages']['menu'] = $kept;
	} else {
		unset($config['installedpackages']['menu']);
	}
}

write_config("Network Monitor: {$action} menu entry");
echo "Network Monitor menu entry: {$action} done\n";
