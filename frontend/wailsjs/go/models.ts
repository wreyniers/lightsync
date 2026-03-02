export namespace capture {
	
	export class MonitorInfo {
	    index: number;
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	    isPrimary: boolean;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new MonitorInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.isPrimary = source["isPrimary"];
	        this.name = source["name"];
	    }
	}
	export class WindowInfo {
	    hwnd: number;
	    title: string;
	    exeName: string;
	
	    static createFrom(source: any = {}) {
	        return new WindowInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hwnd = source["hwnd"];
	        this.title = source["title"];
	        this.exeName = source["exeName"];
	    }
	}

}

export namespace discovery {
	
	export class DiscoveredHueBridge {
	    ip: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredHueBridge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.name = source["name"];
	    }
	}

}

export namespace lights {
	
	export class Color {
	    h: number;
	    s: number;
	    b: number;
	
	    static createFrom(source: any = {}) {
	        return new Color(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.h = source["h"];
	        this.s = source["s"];
	        this.b = source["b"];
	    }
	}
	export class Device {
	    id: string;
	    brand: string;
	    name: string;
	    model?: string;
	    lastIp: string;
	    // Go type: time
	    lastSeen: any;
	    supportsColor: boolean;
	    supportsKelvin: boolean;
	    minKelvin?: number;
	    maxKelvin?: number;
	    kelvinStep?: number;
	    firmwareVersion?: string;
	    room?: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.brand = source["brand"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.lastIp = source["lastIp"];
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.supportsColor = source["supportsColor"];
	        this.supportsKelvin = source["supportsKelvin"];
	        this.minKelvin = source["minKelvin"];
	        this.maxKelvin = source["maxKelvin"];
	        this.kelvinStep = source["kelvinStep"];
	        this.firmwareVersion = source["firmwareVersion"];
	        this.room = source["room"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeviceState {
	    on: boolean;
	    brightness: number;
	    color?: Color;
	    kelvin?: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.on = source["on"];
	        this.brightness = source["brightness"];
	        this.color = this.convertValues(source["color"], Color);
	        this.kelvin = source["kelvin"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class CreateSceneRequest {
	    name: string;
	    trigger: string;
	    devices: Record<string, lights.DeviceState>;
	    globalColor?: lights.Color;
	    globalKelvin?: number;
	    screenSync?: store.ScreenSyncConfig;
	
	    static createFrom(source: any = {}) {
	        return new CreateSceneRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.trigger = source["trigger"];
	        this.devices = this.convertValues(source["devices"], lights.DeviceState, true);
	        this.globalColor = this.convertValues(source["globalColor"], lights.Color);
	        this.globalKelvin = source["globalKelvin"];
	        this.screenSync = this.convertValues(source["screenSync"], store.ScreenSyncConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiscoverResult {
	    devices: lights.Device[];
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiscoverResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.devices = this.convertValues(source["devices"], lights.Device);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PairResult {
	    success: boolean;
	    username: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PairResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.username = source["username"];
	        this.error = source["error"];
	    }
	}
	export class ScreenSyncState {
	    running: boolean;
	    sceneId: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenSyncState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.sceneId = source["sceneId"];
	    }
	}

}

export namespace store {
	
	export class CaptureRect {
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new CaptureRect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class HueBridge {
	    id: string;
	    ip: string;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new HueBridge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ip = source["ip"];
	        this.username = source["username"];
	    }
	}
	export class ScreenSyncConfig {
	    captureMode: string;
	    monitorIndex: number;
	    region: CaptureRect;
	    windowHwnd?: number;
	    windowTitle?: string;
	    deviceIds: string[];
	    colorMode: string;
	    extractionMethod: string;
	    multiColorApproach: string;
	    subMethod: string;
	    paletteStability: number;
	    saturationBoost: number;
	    whiteBias: number;
	    brightnessMode: string;
	    brightnessMultiplier: number;
	    speedPreset: string;
	    assignmentStrategy: string;
	    identityLockBreachThreshold: number;
	    flowTrackEmaAlpha: number;
	    flowTrackSolveIntervalMs: number;
	    sceneCutRemapHoldMs: number;
	    colorSmoothing: number;
	    assignmentHandoffMs: number;
	    brightnessSmoothing: number;
	    brightnessMaxDeviation: number;
	    sceneCutSensitivity: number;
	    sceneCutMode: string;
	    brightnessFloor: number;
	    brightnessCeiling: number;
	
	    static createFrom(source: any = {}) {
	        return new ScreenSyncConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.captureMode = source["captureMode"];
	        this.monitorIndex = source["monitorIndex"];
	        this.region = this.convertValues(source["region"], CaptureRect);
	        this.windowHwnd = source["windowHwnd"];
	        this.windowTitle = source["windowTitle"];
	        this.deviceIds = source["deviceIds"];
	        this.colorMode = source["colorMode"];
	        this.extractionMethod = source["extractionMethod"];
	        this.multiColorApproach = source["multiColorApproach"];
	        this.subMethod = source["subMethod"];
	        this.paletteStability = source["paletteStability"];
	        this.saturationBoost = source["saturationBoost"];
	        this.whiteBias = source["whiteBias"];
	        this.brightnessMode = source["brightnessMode"];
	        this.brightnessMultiplier = source["brightnessMultiplier"];
	        this.speedPreset = source["speedPreset"];
	        this.assignmentStrategy = source["assignmentStrategy"];
	        this.identityLockBreachThreshold = source["identityLockBreachThreshold"];
	        this.flowTrackEmaAlpha = source["flowTrackEmaAlpha"];
	        this.flowTrackSolveIntervalMs = source["flowTrackSolveIntervalMs"];
	        this.sceneCutRemapHoldMs = source["sceneCutRemapHoldMs"];
	        this.colorSmoothing = source["colorSmoothing"];
	        this.assignmentHandoffMs = source["assignmentHandoffMs"];
	        this.brightnessSmoothing = source["brightnessSmoothing"];
	        this.brightnessMaxDeviation = source["brightnessMaxDeviation"];
	        this.sceneCutSensitivity = source["sceneCutSensitivity"];
	        this.sceneCutMode = source["sceneCutMode"];
	        this.brightnessFloor = source["brightnessFloor"];
	        this.brightnessCeiling = source["brightnessCeiling"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Scene {
	    id: string;
	    name: string;
	    trigger: string;
	    devices: Record<string, lights.DeviceState>;
	    globalColor?: lights.Color;
	    globalKelvin?: number;
	    screenSync?: ScreenSyncConfig;
	
	    static createFrom(source: any = {}) {
	        return new Scene(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.trigger = source["trigger"];
	        this.devices = this.convertValues(source["devices"], lights.DeviceState, true);
	        this.globalColor = this.convertValues(source["globalColor"], lights.Color);
	        this.globalKelvin = source["globalKelvin"];
	        this.screenSync = this.convertValues(source["screenSync"], ScreenSyncConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Settings {
	    pollIntervalMs: number;
	    startMinimized: boolean;
	    launchAtLogin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pollIntervalMs = source["pollIntervalMs"];
	        this.startMinimized = source["startMinimized"];
	        this.launchAtLogin = source["launchAtLogin"];
	    }
	}

}

