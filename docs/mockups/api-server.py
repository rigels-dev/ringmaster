#!/usr/bin/env python3
import http.server
import json
import os
import re

PORT = 7890
DIRECTORY = "/home/ubuntu/project/ringmaster/docs/mockups"
CONFIG_FILE = "/home/ubuntu/project/ringmaster/ringmaster.toml"

class CustomHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=DIRECTORY, **kwargs)

    def do_GET(self):
        if self.path == "/api/config":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            
            cfg = self.read_toml_config()
            self.wfile.write(json.dumps(cfg).encode("utf-8"))
            return
        super().do_GET()

    def do_POST(self):
        if self.path == "/api/ping":
            content_length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(content_length).decode("utf-8")
            try:
                data = json.loads(body)
                p_type = data.get("type", "openai")
                base_url = data.get("base_url", "")
                api_key_env = data.get("api_key_env", "")
                model = data.get("model", "")

                res = self.test_ping(p_type, base_url, api_key_env, model)
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                self.wfile.write(json.dumps(res).encode("utf-8"))
            except Exception as e:
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                self.wfile.write(json.dumps({"ok": False, "error": str(e)}).encode("utf-8"))
            return

        if self.path == "/api/config":
            content_length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(content_length).decode("utf-8")
            try:
                data = json.loads(body)
                self.save_toml_config(data)
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                self.wfile.write(json.dumps({"status": "ok", "message": "Saved successfully!"}).encode("utf-8"))
            except Exception as e:
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                self.wfile.write(json.dumps({"status": "error", "message": str(e)}).encode("utf-8"))
            return
        super().do_POST()

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()

    def test_ping(self, p_type, base_url, api_key_env, model):
        import time, urllib.request, urllib.error
        start = time.time()

        if p_type in ["claude_subscription", "claude_sub"]:
            # Check ~/.claude existence or cli
            claude_dir = os.path.expanduser("~/.claude")
            if os.path.exists(claude_dir):
                latency = int((time.time() - start) * 1000)
                return {"ok": True, "latency_ms": latency, "message": f"Claude CLI credentials detected (~/.claude/)"}
            return {"ok": False, "error": "No ~/.claude session found on host. Run 'claude auth login' first."}

        # Resolve API Key
        api_key = os.environ.get(api_key_env, "")
        if not api_key and api_key_env:
            return {"ok": False, "error": f"Environment variable '{api_key_env}' is not set on host"}

        if not base_url:
            if p_type == "anthropic":
                base_url = "https://api.anthropic.com/v1"
            else:
                base_url = "https://api.openai.com/v1"

        try:
            # Query /models or test endpoint
            target_url = base_url.rstrip("/") + "/models"
            req = urllib.request.Request(target_url)
            if api_key:
                if p_type == "anthropic":
                    req.add_header("x-api-key", api_key)
                    req.add_header("anthropic-version", "2023-06-01")
                else:
                    req.add_header("Authorization", f"Bearer {api_key}")

            req.add_header("User-Agent", "Ringmaster/0.4")
            with urllib.request.urlopen(req, timeout=8) as response:
                status_code = response.getcode()
                raw = response.read()
                latency = int((time.time() - start) * 1000)
                try:
                    js = json.loads(raw)
                    model_count = len(js.get("data", js.get("models", [])))
                    return {"ok": True, "latency_ms": latency, "message": f"Connected ({model_count} models reachable)"}
                except:
                    return {"ok": True, "latency_ms": latency, "message": "Endpoint reachable"}
        except urllib.error.HTTPError as e:
            latency = int((time.time() - start) * 1000)
            return {"ok": False, "latency_ms": latency, "error": f"HTTP {e.code}: {e.reason}"}
        except Exception as e:
            latency = int((time.time() - start) * 1000)
            return {"ok": False, "latency_ms": latency, "error": str(e)}

    def read_toml_config(self):
        if not os.path.exists(CONFIG_FILE):
            return {"providers": {}, "agents": {}}
        
        providers = {}
        agents = {}
        current_section = None
        current_sub = None
        
        with open(CONFIG_FILE, "r") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                sec_match = re.match(r"^\[([a-zA-Z0-9_\-\.]+)]$", line)
                if sec_match:
                    parts = sec_match.group(1).split(".")
                    current_section = parts[0]
                    current_sub = parts[1] if len(parts) > 1 else None
                    if current_section == "providers" and current_sub:
                        providers[current_sub] = {}
                    elif current_section == "agents" and current_sub:
                        agents[current_sub] = {}
                    continue
                
                kv_match = re.match(r"^([a-zA-Z0-9_\-]+)\s*=\s*(.+)$", line)
                if kv_match and current_section and current_sub:
                    k = kv_match.group(1)
                    v = kv_match.group(2).strip().strip('"').strip("'")
                    if v.lower() == "true": v = True
                    elif v.lower() == "false": v = False
                    
                    if current_section == "providers":
                        providers[current_sub][k] = v
                    elif current_section == "agents":
                        agents[current_sub][k] = v

        return {"providers": providers, "agents": agents}

    def save_toml_config(self, data):
        providers = data.get("providers", {})
        agents = data.get("agents", {})

        out = []
        for name, p in providers.items():
            out.append(f"[providers.{name}]")
            if p.get("type"):
                out.append(f'type = "{p.get("type")}"')
            if p.get("base_url"):
                out.append(f'base_url = "{p.get("base_url", "")}"')
            if p.get("api_key_env"):
                out.append(f'api_key_env = "{p.get("api_key_env", "")}"')
            out.append("")

        for alias, a in agents.items():
            out.append(f"[agents.{alias}]")
            out.append(f'provider = "{a.get("provider", "")}"')
            out.append(f'model = "{a.get("model", "")}"')
            out.append(f'role = "{a.get("role", "worker")}"')
            tc = "true" if a.get("tool_calling") else "false"
            out.append(f"tool_calling = {tc}")
            out.append("")

        out.append("[budget]")
        out.append("max_tokens_per_project = 2000000")
        out.append("max_tokens_per_task = 300000")
        out.append("")

        with open(CONFIG_FILE, "w") as f:
            f.write("\n".join(out))

if __name__ == "__main__":
    server = http.server.ThreadingHTTPServer(("", PORT), CustomHandler)
    server.serve_forever()
