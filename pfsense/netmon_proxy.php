<?php
/*
 * Forwards requests from the pfSense web GUI to the Network Monitor running on
 * this firewall (installed to /usr/local/www by install.sh).
 *
 * Including guiconfig.inc means every request needs a logged-in pfSense admin,
 * and POSTs must carry pfSense's CSRF token (the dashboard sends it).
 */

require_once("guiconfig.inc");

// Keep in sync with NETMON_DATA_DIR in install.sh.
$netmon_config_file = "/usr/local/etc/netmon/config.json";

$settings = @json_decode(@file_get_contents($netmon_config_file), true);
if (!is_array($settings)) {
	$settings = array();
}

// Work out where the monitor listens: ":8080" or "0.0.0.0:8080" means localhost.
$listen = isset($settings['listen']) ? $settings['listen'] : ':8080';
$sep = strrpos($listen, ':');
$host = trim(substr($listen, 0, $sep), '[]');
$port = intval(substr($listen, $sep + 1));
if ($host === '' || $host === '0.0.0.0' || $host === '::') {
	$host = '127.0.0.1';
}
if (strpos($host, ':') !== false) {
	$host = '[' . $host . ']';
}

// Only the dashboard, its static files and the JSON API may be requested.
$path = isset($_GET['p']) ? $_GET['p'] : '';
if (!preg_match('#^(api/[a-z]+|static/[A-Za-z0-9_./-]+)?$#', $path) || strpos($path, '..') !== false) {
	http_response_code(400);
	echo "Bad path";
	exit;
}
$query = $_GET;
unset($query['p']);
$url = "http://{$host}:{$port}/" . $path . (count($query) ? '?' . http_build_query($query) : '');

$headers = array();
$ch = curl_init($url);
curl_setopt_array($ch, array(
	CURLOPT_RETURNTRANSFER => true,
	CURLOPT_CONNECTTIMEOUT => 5,
	CURLOPT_TIMEOUT => 120,
	CURLOPT_HEADERFUNCTION => function ($ch, $line) use (&$headers) {
		$parts = explode(':', $line, 2);
		if (count($parts) == 2) {
			$headers[strtolower(trim($parts[0]))] = trim($parts[1]);
		}
		return strlen($line);
	},
));
if (!empty($settings['ui_password'])) {
	curl_setopt($ch, CURLOPT_USERPWD, 'pfsense:' . $settings['ui_password']);
}
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
	// The JSON body arrives as a form field so pfSense's CSRF check can see the token.
	curl_setopt($ch, CURLOPT_POST, true);
	curl_setopt($ch, CURLOPT_POSTFIELDS, isset($_POST['body']) ? $_POST['body'] : '{}');
	curl_setopt($ch, CURLOPT_HTTPHEADER, array('Content-Type: application/json'));
}

$response = curl_exec($ch);
if ($response === false) {
	$error = curl_error($ch);
	curl_close($ch);
	http_response_code(502);
	header('Content-Type: application/json');
	echo json_encode(array('error' => "Network Monitor isn't running ({$error}). Start it with: service netmon.sh start"));
	exit;
}
$status = curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
curl_close($ch);

http_response_code($status);
foreach (array('Content-Type', 'Content-Disposition') as $name) {
	if (isset($headers[strtolower($name)])) {
		header($name . ': ' . $headers[strtolower($name)]);
	}
}

if ($path === '') {
	// Route the page's own files back through this proxy and hand it the CSRF token.
	$token = function_exists('csrf_get_tokens') ? csrf_get_tokens() : '';
	$token_name = isset($GLOBALS['csrf']['input-name']) ? $GLOBALS['csrf']['input-name'] : '__csrf_magic';
	$inject = '<script>window.NETMON_PROXY = ' . json_encode(array(
		'base' => 'netmon_proxy.php?p=',
		'csrf' => $token,
		'csrfName' => $token_name,
	)) . ';</script>';
	$response = str_replace(
		array('href="static/', 'src="static/'),
		array('href="netmon_proxy.php?p=static/', 'src="netmon_proxy.php?p=static/'),
		$response
	);
	$response = str_replace('</head>', $inject . "\n</head>", $response);
}

echo $response;
