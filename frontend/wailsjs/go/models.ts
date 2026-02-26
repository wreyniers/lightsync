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

}

export namespace store {
	
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
	export class Scene {
	    id: string;
	    name: string;
	    trigger: string;
	    devices: Record<string, lights.DeviceState>;
	    globalColor?: lights.Color;
	    globalKelvin?: number;
	
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

