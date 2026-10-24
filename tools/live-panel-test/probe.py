"""Opt-in Application API probe for the one approved Windows Native Demo."""
import argparse
import getpass
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

DEMO_UUID = "58e8c7f9-4923-43b3-b431-90b2bf4edf4f"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def require(value):
    if not value:
        raise RuntimeError("Panel probe check failed; sensitive details omitted")


def run(key, panel, node, node_id, server_id, allow_demo_lifecycle=False):
    checks = {}
    report = {"schema_version": 1, "observed_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "identity_source": "real panel Application API key and panel-provided node token",
              "browser_tested": False, "client_api_key_used": False,
              "checks": checks, "passed": False}
    was_running = False
    restore_needed = False
    native = None
    failed = False
    try:
        require(allow_demo_lifecycle and key.startswith("ptla_") and len(key) <= 1024)
        for base in (panel, node):
            u = urllib.parse.urlsplit(base)
            require(u.scheme == "https" and u.hostname and not u.username and not u.password
                    and not u.query and not u.fragment and not u.path and u.port in (None, 443))
        require(type(node_id) is int and node_id > 0 and type(server_id) is int and server_id > 0)
        opener = urllib.request.build_opener(NoRedirect)

        def call(base, token, method, route, body=None):
            data = json.dumps(body).encode() if body is not None else None
            request = urllib.request.Request(base + route, data=data, method=method,
                headers={"Authorization": "Bearer " + token, "Accept": "application/json",
                         "Content-Type": "application/json", "User-Agent": "Wings-Windows-Panel-Probe"})
            try:
                with opener.open(request, timeout=15) as response:
                    raw = response.read((2 << 20) + 1)
                    require(len(raw) <= 2 << 20)
                    return response.status, raw
            except urllib.error.HTTPError as error:
                # No error body or token is included in a report/exception.
                status = error.code
                error.close()
                return status, b""

        def app(method, route, body=None):
            return call(panel, key, method, "/api/application" + route, body)

        def app_server():
            status, raw = app("GET", "/servers/" + str(server_id))
            require(status == 200)
            value = json.loads(raw)["attributes"]
            require(value["uuid"] == DEMO_UUID and value["node"] == node_id)
            return value

        details = app_server()
        require(not details.get("suspended") and details.get("status") is None)
        checks["application_server_matches_approved_demo"] = True
        status, raw = app("GET", "/nodes/" + str(node_id))
        require(status == 200)
        node_details = json.loads(raw)["attributes"]
        require(node_details["fqdn"] == urllib.parse.urlsplit(node).hostname
                and node_details["scheme"] == "https" and node_details["daemon_listen"] == 443)
        checks["application_node_matches_explicit_https_endpoint"] = True
        status, raw = app("GET", "/nodes/" + str(node_id) + "/configuration")
        require(status == 200)
        config = json.loads(raw)
        node_token = config["token"]
        require(isinstance(node_token, str) and len(node_token) >= 32)
        config = None
        raw = None

        def native(method, suffix="", body=None):
            return call(node, node_token, method, "/api/servers/" + DEMO_UUID + suffix, body)

        def current():
            status, raw = native("GET")
            require(status == 200)
            return json.loads(raw)

        def wait(predicate, seconds=45):
            until = time.monotonic() + seconds
            while time.monotonic() < until:
                value = current()
                if predicate(value):
                    return value
                time.sleep(0.2)
            raise RuntimeError("Panel probe timed out; sensitive details omitted")

        before = current()
        require(not before["is_suspended"] and before["native_install"]["enabled"]
                and not before["native_install"]["active"])
        was_running = before["state"] == "running"
        require(was_running)  # This probe only operates on the established running demo.
        checks["panel_configuration_token_authenticates_live_node"] = True

        # An Application key must not silently gain Client API permissions.
        status, _ = call(panel, key, "GET", "/api/client/servers/58e8c7f9/resources")
        require(status == 403)
        checks["application_key_refused_by_client_api"] = True

        restore_needed = True
        status, _ = app("POST", "/servers/" + str(server_id) + "/suspend", {})
        require(status == 204)
        wait(lambda value: value["is_suspended"] and value["state"] == "offline")
        require(app_server().get("suspended") is True)
        checks["real_panel_suspension_syncs_and_stops_demo"] = True
        status, _ = native("POST", "/power", {"action": "start", "wait_seconds": 5})
        require(status == 400)
        checks["suspended_start_refused"] = True
        status, _ = app("POST", "/servers/" + str(server_id) + "/unsuspend", {})
        require(status == 204)
        wait(lambda value: not value["is_suspended"] and value["state"] == "offline")
        require(not app_server().get("suspended"))
        checks["real_panel_unsuspension_syncs"] = True

        def marker_modified():
            status, raw = native("GET", "/files/list-directory?directory=/")
            require(status == 200)
            marker = [entry for entry in json.loads(raw) if entry["name"] == "install.marker"]
            require(len(marker) == 1)
            return marker[0]["modified"]

        previous_marker = marker_modified()
        status, _ = app("POST", "/servers/" + str(server_id) + "/reinstall", {})
        require(status == 204)
        value = wait(lambda value: not value["native_install"]["active"]
                     and value["native_install"]["stage"] == "complete"
                     and value["native_install"]["successful"]
                     and not value["native_install"]["callback_error"])
        require(value["state"] == "offline" and marker_modified() != previous_marker)
        status, raw = native("GET", "/files/contents?file=/install.marker")
        require(status == 200 and raw == b"installed")
        checks["real_panel_reinstall_runs_approved_recipe_and_rewrites_marker"] = True
        deadline = time.monotonic() + 15
        while app_server().get("status") is not None and time.monotonic() < deadline:
            time.sleep(0.2)
        require(app_server().get("status") is None)
        checks["actual_install_callback_clears_panel_installing_state"] = True
        require(not value["native_install"]["pending_notification"])
        checks["installation_finishes_without_pending_notification"] = True

        status, _ = native("POST", "/power", {"action": "start", "wait_seconds": 5})
        require(status == 202)
        wait(lambda value: value["state"] == "running")
        nonce = "panel-api-" + uuid.uuid4().hex[:16]
        status, _ = native("POST", "/commands", {"commands": [nonce]})
        require(status == 204)
        deadline = time.monotonic() + 10
        echoed = False
        while time.monotonic() < deadline:
            status, raw = native("GET", "/logs?size=100")
            require(status == 200)
            echoed = ("ECHO:" + nonce).encode() in raw
            if echoed:
                break
            time.sleep(0.2)
        require(echoed)
        checks["demo_restarts_and_echoes_unique_console_command"] = True
    except Exception:
        failed = True
        report["error"] = "Panel probe failed; sensitive details omitted"
    finally:
        if restore_needed and native is not None:
            try:
                status, _ = app("POST", "/servers/" + str(server_id) + "/unsuspend", {})
                require(status == 204)
                wait(lambda value: not value["is_suspended"] and not value["native_install"]["active"])
                if was_running and current()["state"] != "running":
                    status, _ = native("POST", "/power", {"action": "start", "wait_seconds": 5})
                    require(status == 202)
                value = wait(lambda value: value["state"] == "running")
                require(not app_server().get("suspended") and app_server().get("status") is None)
                checks["demo_and_panel_restored_to_running_unsuspended"] = True
            except Exception:
                failed = True
                report["recovery_error"] = "Demo restoration requires inspection; sensitive details omitted"
        report["passed"] = not failed
        report["finished_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("panel")
    parser.add_argument("node")
    parser.add_argument("node_id", type=int)
    parser.add_argument("server_id", type=int)
    parser.add_argument("--allow-demo-lifecycle", action="store_true")
    args = parser.parse_args()
    key = getpass.getpass("Temporary Application key: ") if sys.stdin.isatty() else sys.stdin.readline(1025).strip()
    result = run(key, args.panel, args.node, args.node_id, args.server_id, args.allow_demo_lifecycle)
    key = None
    print(json.dumps(result))
    sys.exit(0 if result["passed"] else 1)
