export namespace app {
	
	export class BranchSignal {
	    name: string;
	    sha: string;
	    local: boolean;
	    author: string;
	    lastCommit: number;
	    firstCommit: number;
	    behind: number;
	    added: number;
	    removed: number;
	    paths: string[];
	    staleDays: number;
	    ageDays: number;
	    flags: string[];
	
	    static createFrom(source: any = {}) {
	        return new BranchSignal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sha = source["sha"];
	        this.local = source["local"];
	        this.author = source["author"];
	        this.lastCommit = source["lastCommit"];
	        this.firstCommit = source["firstCommit"];
	        this.behind = source["behind"];
	        this.added = source["added"];
	        this.removed = source["removed"];
	        this.paths = source["paths"];
	        this.staleDays = source["staleDays"];
	        this.ageDays = source["ageDays"];
	        this.flags = source["flags"];
	    }
	}
	export class Thresholds {
	    stale: number;
	    longLived: number;
	    behind: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Thresholds(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stale = source["stale"];
	        this.longLived = source["longLived"];
	        this.behind = source["behind"];
	        this.size = source["size"];
	    }
	}
	export class InsightsWindow {
	    target: string;
	    targetSha: string;
	    asOf: number;
	    since: number;
	    until: number;
	    range: string;
	    points?: string;
	    kinds: Record<string, Array<string>>;
	    references: string[];
	
	    static createFrom(source: any = {}) {
	        return new InsightsWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.targetSha = source["targetSha"];
	        this.asOf = source["asOf"];
	        this.since = source["since"];
	        this.until = source["until"];
	        this.range = source["range"];
	        this.points = source["points"];
	        this.kinds = source["kinds"];
	        this.references = source["references"];
	    }
	}
	export class AttentionResult {
	    window: InsightsWindow;
	    thresholds: Thresholds;
	    branches: BranchSignal[];
	
	    static createFrom(source: any = {}) {
	        return new AttentionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window = this.convertValues(source["window"], InsightsWindow);
	        this.thresholds = this.convertValues(source["thresholds"], Thresholds);
	        this.branches = this.convertValues(source["branches"], BranchSignal);
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
	export class BatchResult {
	    path: string;
	    ok: boolean;
	    message: string;
	    skipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.skipped = source["skipped"];
	    }
	}
	export class BisectMark {
	    commit: string;
	    subject: string;
	    term: string;
	    step: number;
	
	    static createFrom(source: any = {}) {
	        return new BisectMark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.subject = source["subject"];
	        this.term = source["term"];
	        this.step = source["step"];
	    }
	}
	export class BisectInfo {
	    running: boolean;
	    current: string;
	    marks: BisectMark[];
	    steps: number;
	    remaining: number;
	    stepsLeft: number;
	    firstBad: string;
	    candidates: string[];
	    origin: string;
	    inQuestion: string[];
	    rangeCapped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BisectInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.current = source["current"];
	        this.marks = this.convertValues(source["marks"], BisectMark);
	        this.steps = source["steps"];
	        this.remaining = source["remaining"];
	        this.stepsLeft = source["stepsLeft"];
	        this.firstBad = source["firstBad"];
	        this.candidates = source["candidates"];
	        this.origin = source["origin"];
	        this.inQuestion = source["inQuestion"];
	        this.rangeCapped = source["rangeCapped"];
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
	
	export class BisectPlan {
	    bad: string;
	    good: string;
	    badSubject: string;
	    goodSubject: string;
	    suggested: string;
	    commits: number;
	    steps: number;
	    first: string;
	    firstSubject: string;
	    problem: string;
	
	    static createFrom(source: any = {}) {
	        return new BisectPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bad = source["bad"];
	        this.good = source["good"];
	        this.badSubject = source["badSubject"];
	        this.goodSubject = source["goodSubject"];
	        this.suggested = source["suggested"];
	        this.commits = source["commits"];
	        this.steps = source["steps"];
	        this.first = source["first"];
	        this.firstSubject = source["firstSubject"];
	        this.problem = source["problem"];
	    }
	}
	export class BranchLoss {
	    branch: string;
	    upstream: string;
	    commits: git.CommitLine[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new BranchLoss(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.upstream = source["upstream"];
	        this.commits = this.convertValues(source["commits"], git.CommitLine);
	        this.total = source["total"];
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
	
	export class CommitInfo {
	    commit: git.CommitDetail;
	    files: git.FileChange[];
	
	    static createFrom(source: any = {}) {
	        return new CommitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = this.convertValues(source["commit"], git.CommitDetail);
	        this.files = this.convertValues(source["files"], git.FileChange);
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
	export class ConfigProblem {
	    line: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigProblem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.message = source["message"];
	    }
	}
	export class ConfigDoc {
	    path: string;
	    text: string;
	    entries: git.ConfigEntry[];
	    exists: boolean;
	    dirty: boolean;
	    changedOnDisk: boolean;
	    problem?: ConfigProblem;
	
	    static createFrom(source: any = {}) {
	        return new ConfigDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.text = source["text"];
	        this.entries = this.convertValues(source["entries"], git.ConfigEntry);
	        this.exists = source["exists"];
	        this.dirty = source["dirty"];
	        this.changedOnDisk = source["changedOnDisk"];
	        this.problem = this.convertValues(source["problem"], ConfigProblem);
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
	
	export class ConfigSaveResult {
	    saved: boolean;
	    doc: ConfigDoc;
	    diskText: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigSaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	        this.doc = this.convertValues(source["doc"], ConfigDoc);
	        this.diskText = source["diskText"];
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
	export class ConfigScopes {
	    global: string;
	    globalAlt: string;
	    local: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigScopes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global = source["global"];
	        this.globalAlt = source["globalAlt"];
	        this.local = source["local"];
	    }
	}
	export class Environment {
	    gitPath: string;
	    gitVersion: string;
	    lfsVersion: string;
	    problem: string;
	    minVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gitPath = source["gitPath"];
	        this.gitVersion = source["gitVersion"];
	        this.lfsVersion = source["lfsVersion"];
	        this.problem = source["problem"];
	        this.minVersion = source["minVersion"];
	    }
	}
	export class FailedRepo {
	    path: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new FailedRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class GraphPage {
	    rows: graph.Row[];
	    refs: Record<string, Array<git.Ref>>;
	    total: number;
	    loading: boolean;
	    paused: boolean;
	    estimate: number;
	
	    static createFrom(source: any = {}) {
	        return new GraphPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], graph.Row);
	        this.refs = this.convertValues(source["refs"], Array<git.Ref>, true);
	        this.total = source["total"];
	        this.loading = source["loading"];
	        this.paused = source["paused"];
	        this.estimate = source["estimate"];
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
	export class GroupRepo {
	    path: string;
	    name: string;
	    exists: boolean;
	    branch: string;
	    upstream: string;
	    ahead: number;
	    behind: number;
	    changes: number;
	    conflicts: number;
	    lastFetch: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new GroupRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.exists = source["exists"];
	        this.branch = source["branch"];
	        this.upstream = source["upstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.changes = source["changes"];
	        this.conflicts = source["conflicts"];
	        this.lastFetch = source["lastFetch"];
	        this.error = source["error"];
	    }
	}
	export class HotFile {
	    path: string;
	    lines: number;
	    items: number;
	    openBranches: number;
	
	    static createFrom(source: any = {}) {
	        return new HotFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.lines = source["lines"];
	        this.items = source["items"];
	        this.openBranches = source["openBranches"];
	    }
	}
	export class HotspotsResult {
	    window: InsightsWindow;
	    files: HotFile[];
	    rework: git.ShippedItem[];
	
	    static createFrom(source: any = {}) {
	        return new HotspotsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window = this.convertValues(source["window"], InsightsWindow);
	        this.files = this.convertValues(source["files"], HotFile);
	        this.rework = this.convertValues(source["rework"], git.ShippedItem);
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
	export class IgnoreAdded {
	    rule: git.IgnoreLine;
	    hidden: number;
	    untracked: string[];
	
	    static createFrom(source: any = {}) {
	        return new IgnoreAdded(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rule = this.convertValues(source["rule"], git.IgnoreLine);
	        this.hidden = source["hidden"];
	        this.untracked = source["untracked"];
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
	export class IgnoreFiles {
	    global: string;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreFiles(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global = source["global"];
	    }
	}
	export class Ignored {
	    files: git.IgnoredFile[];
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Ignored(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], git.IgnoredFile);
	        this.truncated = source["truncated"];
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
	export class InsightsInputs {
	    target: string;
	    window: string;
	    since: number;
	    until: number;
	    from: string;
	    to: string;
	    asOf: number;
	    thresholds: Thresholds;
	
	    static createFrom(source: any = {}) {
	        return new InsightsInputs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.window = source["window"];
	        this.since = source["since"];
	        this.until = source["until"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.asOf = source["asOf"];
	        this.thresholds = this.convertValues(source["thresholds"], Thresholds);
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
	
	export class KindPatterns {
	    kind: string;
	    patterns: string[];
	    custom: boolean;
	    defaults: string[];
	
	    static createFrom(source: any = {}) {
	        return new KindPatterns(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.patterns = source["patterns"];
	        this.custom = source["custom"];
	        this.defaults = source["defaults"];
	    }
	}
	export class LFSInfo {
	    available: boolean;
	    status: git.LFSStatus;
	
	    static createFrom(source: any = {}) {
	        return new LFSInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.status = this.convertValues(source["status"], git.LFSStatus);
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
	export class LinkTry {
	    scanned: number;
	    commits: number;
	    matches: links.Match[];
	    problem?: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkTry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanned = source["scanned"];
	        this.commits = source["commits"];
	        this.matches = this.convertValues(source["matches"], links.Match);
	        this.problem = source["problem"];
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
	export class Preferences {
	    theme: string;
	    graphDensity: string;
	    updateChecks: boolean;
	    gitPath: string;
	    gitInUse: string;
	    cloneDir: string;
	    zoom: number;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.graphDensity = source["graphDensity"];
	        this.updateChecks = source["updateChecks"];
	        this.gitPath = source["gitPath"];
	        this.gitInUse = source["gitInUse"];
	        this.cloneDir = source["cloneDir"];
	        this.zoom = source["zoom"];
	    }
	}
	export class RecentRepo {
	    path: string;
	    name: string;
	    // Go type: time
	    lastOpened: any;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RecentRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.lastOpened = this.convertValues(source["lastOpened"], null);
	        this.exists = source["exists"];
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
	export class ReleaseBranch {
	    name: string;
	    forkTime: number;
	
	    static createFrom(source: any = {}) {
	        return new ReleaseBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.forkTime = source["forkTime"];
	    }
	}
	export class RepoPreferences {
	    kinds: KindPatterns[];
	    hide: string[];
	    hideKinds: string[];
	    cleanupOnPull: boolean;
	    syncNotes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoPreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kinds = this.convertValues(source["kinds"], KindPatterns);
	        this.hide = source["hide"];
	        this.hideKinds = source["hideKinds"];
	        this.cleanupOnPull = source["cleanupOnPull"];
	        this.syncNotes = source["syncNotes"];
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
	export class RepoState {
	    operation: string;
	    step: number;
	    total: number;
	    actions: string[];
	    conflicts: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepoState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation = source["operation"];
	        this.step = source["step"];
	        this.total = source["total"];
	        this.actions = source["actions"];
	        this.conflicts = source["conflicts"];
	    }
	}
	export class RepoTab {
	    path: string;
	    name: string;
	    head: string;
	
	    static createFrom(source: any = {}) {
	        return new RepoTab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.head = source["head"];
	    }
	}
	export class SessionState {
	    tabs: RepoTab[];
	    active: string;
	    failed: FailedRepo[];
	    configTab: string;
	    configTabId: string;
	    insightsTab: string;
	    insightsTabId: string;
	    ignoreTab: string;
	    ignoreTabId: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tabs = this.convertValues(source["tabs"], RepoTab);
	        this.active = source["active"];
	        this.failed = this.convertValues(source["failed"], FailedRepo);
	        this.configTab = source["configTab"];
	        this.configTabId = source["configTabId"];
	        this.insightsTab = source["insightsTab"];
	        this.insightsTabId = source["insightsTabId"];
	        this.ignoreTab = source["ignoreTab"];
	        this.ignoreTabId = source["ignoreTabId"];
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
	export class SharedCommits {
	    upstream: string;
	    commits: git.CommitLine[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new SharedCommits(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upstream = source["upstream"];
	        this.commits = this.convertValues(source["commits"], git.CommitLine);
	        this.total = source["total"];
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
	export class ShippedResult {
	    window: InsightsWindow;
	    items: git.ShippedItem[];
	
	    static createFrom(source: any = {}) {
	        return new ShippedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window = this.convertValues(source["window"], InsightsWindow);
	        this.items = this.convertValues(source["items"], git.ShippedItem);
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
	export class UnmergedBranch {
	    name: string;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new UnmergedBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.commits = source["commits"];
	    }
	}
	export class StackPlan {
	    delete: string[];
	    keep: string[];
	    unmerged: UnmergedBranch[];
	
	    static createFrom(source: any = {}) {
	        return new StackPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.delete = source["delete"];
	        this.keep = source["keep"];
	        this.unmerged = this.convertValues(source["unmerged"], UnmergedBranch);
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
	export class StackPush {
	    push: string[];
	    merged: string[];
	    losses: BranchLoss[];
	
	    static createFrom(source: any = {}) {
	        return new StackPush(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.push = source["push"];
	        this.merged = source["merged"];
	        this.losses = this.convertValues(source["losses"], BranchLoss);
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
	
	export class UndoInfo {
	    undo: string;
	    redo: string;
	
	    static createFrom(source: any = {}) {
	        return new UndoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.undo = source["undo"];
	        this.redo = source["redo"];
	    }
	}
	
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    url: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.url = source["url"];
	        this.available = source["available"];
	    }
	}
	export class WorkingTree {
	    status: git.Status;
	    signing: git.Signing;
	    identity: git.Identity;
	
	    static createFrom(source: any = {}) {
	        return new WorkingTree(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], git.Status);
	        this.signing = this.convertValues(source["signing"], git.Signing);
	        this.identity = this.convertValues(source["identity"], git.Identity);
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

export namespace git {
	
	export class BlameLine {
	    commit: string;
	    author: string;
	    time: number;
	    summary: string;
	    line: string;
	    origLine: number;
	
	    static createFrom(source: any = {}) {
	        return new BlameLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.author = source["author"];
	        this.time = source["time"];
	        this.summary = source["summary"];
	        this.line = source["line"];
	        this.origLine = source["origLine"];
	    }
	}
	export class BranchMerge {
	    target: string;
	    how: string;
	    unmerged: number;
	
	    static createFrom(source: any = {}) {
	        return new BranchMerge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.how = source["how"];
	        this.unmerged = source["unmerged"];
	    }
	}
	export class Trailer {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Trailer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Signature {
	    name: string;
	    email: string;
	    time: number;
	    tz: string;
	
	    static createFrom(source: any = {}) {
	        return new Signature(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.time = source["time"];
	        this.tz = source["tz"];
	    }
	}
	export class CommitDetail {
	    id: string;
	    parents: string[];
	    author: Signature;
	    committer: Signature;
	    subject: string;
	    body: string;
	    trailers: Trailer[];
	    signed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CommitDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parents = source["parents"];
	        this.author = this.convertValues(source["author"], Signature);
	        this.committer = this.convertValues(source["committer"], Signature);
	        this.subject = source["subject"];
	        this.body = source["body"];
	        this.trailers = this.convertValues(source["trailers"], Trailer);
	        this.signed = source["signed"];
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
	export class CommitLine {
	    id: string;
	    short: string;
	    author: string;
	    subject: string;
	    merge: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CommitLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.short = source["short"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	        this.merge = source["merge"];
	    }
	}
	export class ConfigEntry {
	    scope: string;
	    file: string;
	    key: string;
	    value: string;
	    implicit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.file = source["file"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.implicit = source["implicit"];
	    }
	}
	export class Side {
	    exists: boolean;
	    text: string;
	    image?: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Side(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.text = source["text"];
	        this.image = source["image"];
	        this.size = source["size"];
	    }
	}
	export class ConflictFile {
	    path: string;
	    base: Side;
	    ours: Side;
	    theirs: Side;
	    merged: Side;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConflictFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.base = this.convertValues(source["base"], Side);
	        this.ours = this.convertValues(source["ours"], Side);
	        this.theirs = this.convertValues(source["theirs"], Side);
	        this.merged = this.convertValues(source["merged"], Side);
	        this.binary = source["binary"];
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
	export class FileChange {
	    status: string;
	    path: string;
	    oldPath?: string;
	    added: number;
	    deleted: number;
	    binary?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.path = source["path"];
	        this.oldPath = source["oldPath"];
	        this.added = source["added"];
	        this.deleted = source["deleted"];
	        this.binary = source["binary"];
	    }
	}
	export class FileCommit {
	    id: string;
	    author: string;
	    time: number;
	    subject: string;
	    path: string;
	    oldPath?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new FileCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.author = source["author"];
	        this.time = source["time"];
	        this.subject = source["subject"];
	        this.path = source["path"];
	        this.oldPath = source["oldPath"];
	        this.status = source["status"];
	    }
	}
	export class FileContents {
	    path: string;
	    old: Side;
	    new: Side;
	    kind: string;
	    mime?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileContents(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.old = this.convertValues(source["old"], Side);
	        this.new = this.convertValues(source["new"], Side);
	        this.kind = source["kind"];
	        this.mime = source["mime"];
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
	export class FileStatus {
	    path: string;
	    origPath?: string;
	    index: string;
	    workTree: string;
	    untracked: boolean;
	    conflicted: boolean;
	    submodule: boolean;
	    added: number;
	    deleted: number;
	    binary?: boolean;
	    caseClash?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.origPath = source["origPath"];
	        this.index = source["index"];
	        this.workTree = source["workTree"];
	        this.untracked = source["untracked"];
	        this.conflicted = source["conflicted"];
	        this.submodule = source["submodule"];
	        this.added = source["added"];
	        this.deleted = source["deleted"];
	        this.binary = source["binary"];
	        this.caseClash = source["caseClash"];
	    }
	}
	export class Identity {
	    name: string;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new Identity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	    }
	}
	export class IgnoreLine {
	    source: string;
	    line: number;
	    pattern: string;
	    created?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.line = source["line"];
	        this.pattern = source["pattern"];
	        this.created = source["created"];
	    }
	}
	export class IgnoreCheck {
	    matched: boolean;
	    ignored: boolean;
	    rule: IgnoreLine;
	    tracked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.matched = source["matched"];
	        this.ignored = source["ignored"];
	        this.rule = this.convertValues(source["rule"], IgnoreLine);
	        this.tracked = source["tracked"];
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
	
	export class IgnorePreview {
	    hidden: string[];
	    tracked: string[];
	
	    static createFrom(source: any = {}) {
	        return new IgnorePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hidden = source["hidden"];
	        this.tracked = source["tracked"];
	    }
	}
	export class IgnoreRuleStats {
	    source: string;
	    line: number;
	    ignores: number;
	    restores: number;
	    tracked: number;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new IgnoreRuleStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.line = source["line"];
	        this.ignores = source["ignores"];
	        this.restores = source["restores"];
	        this.tracked = source["tracked"];
	        this.paths = source["paths"];
	    }
	}
	export class IgnoreSource {
	    source: string;
	    path: string;
	    scope: string;
	    dir?: string;
	    exists: boolean;
	    rules: number;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.path = source["path"];
	        this.scope = source["scope"];
	        this.dir = source["dir"];
	        this.exists = source["exists"];
	        this.rules = source["rules"];
	    }
	}
	export class IgnoreStats {
	    rules: IgnoreRuleStats[];
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], IgnoreRuleStats);
	        this.truncated = source["truncated"];
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
	export class IgnoreTarget {
	    scope: string;
	    dir?: string;
	
	    static createFrom(source: any = {}) {
	        return new IgnoreTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.dir = source["dir"];
	    }
	}
	export class IgnoredFile {
	    path: string;
	    rule: IgnoreLine;
	    canExcept: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IgnoredFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.rule = this.convertValues(source["rule"], IgnoreLine);
	        this.canExcept = source["canExcept"];
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
	export class LFSFile {
	    name: string;
	    size: number;
	    downloaded: boolean;
	    oid: string;
	
	    static createFrom(source: any = {}) {
	        return new LFSFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.downloaded = source["downloaded"];
	        this.oid = source["oid"];
	    }
	}
	export class LFSLock {
	    id: string;
	    path: string;
	    owner: string;
	    lockedAt: string;
	    ours: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LFSLock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.owner = source["owner"];
	        this.lockedAt = source["lockedAt"];
	        this.ours = source["ours"];
	    }
	}
	export class LFSPattern {
	    pattern: string;
	    source: string;
	    lockable: boolean;
	    tracked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LFSPattern(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pattern = source["pattern"];
	        this.source = source["source"];
	        this.lockable = source["lockable"];
	        this.tracked = source["tracked"];
	    }
	}
	export class LFSStatus {
	    patterns: LFSPattern[];
	    files: LFSFile[];
	
	    static createFrom(source: any = {}) {
	        return new LFSStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.patterns = this.convertValues(source["patterns"], LFSPattern);
	        this.files = this.convertValues(source["files"], LFSFile);
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
	export class Note {
	    ref: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.text = source["text"];
	    }
	}
	export class RebaseItem {
	    commit: string;
	    subject: string;
	    message: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new RebaseItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.subject = source["subject"];
	        this.message = source["message"];
	        this.action = source["action"];
	    }
	}
	export class Ref {
	    name: string;
	    short: string;
	    kind: string;
	    commit: string;
	    head: boolean;
	    upstream: string;
	    ahead: number;
	    behind: number;
	    gone: boolean;
	    worktree?: string;
	    hidden?: boolean;
	    focused?: boolean;
	    branchKind?: string;
	    stackParent?: string;
	    stackDepth?: number;
	    stackSize?: number;
	    stackParentMerged?: boolean;
	    subject?: string;
	    updated?: number;
	
	    static createFrom(source: any = {}) {
	        return new Ref(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.short = source["short"];
	        this.kind = source["kind"];
	        this.commit = source["commit"];
	        this.head = source["head"];
	        this.upstream = source["upstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.gone = source["gone"];
	        this.worktree = source["worktree"];
	        this.hidden = source["hidden"];
	        this.focused = source["focused"];
	        this.branchKind = source["branchKind"];
	        this.stackParent = source["stackParent"];
	        this.stackDepth = source["stackDepth"];
	        this.stackSize = source["stackSize"];
	        this.stackParentMerged = source["stackParentMerged"];
	        this.subject = source["subject"];
	        this.updated = source["updated"];
	    }
	}
	export class ReflogEntry {
	    selector: string;
	    commit: string;
	    action: string;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new ReflogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selector = source["selector"];
	        this.commit = source["commit"];
	        this.action = source["action"];
	        this.time = source["time"];
	    }
	}
	export class Remote {
	    name: string;
	    fetchUrl: string;
	    pushUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Remote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fetchUrl = source["fetchUrl"];
	        this.pushUrl = source["pushUrl"];
	    }
	}
	export class ShippedItem {
	    sha: string;
	    title: string;
	    author: string;
	    email: string;
	    committed: number;
	    merge: boolean;
	    files: number;
	    added: number;
	    removed: number;
	    refs: string[];
	    cycle: number;
	    revert: boolean;
	    fixup: boolean;
	    bot: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShippedItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sha = source["sha"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.email = source["email"];
	        this.committed = source["committed"];
	        this.merge = source["merge"];
	        this.files = source["files"];
	        this.added = source["added"];
	        this.removed = source["removed"];
	        this.refs = source["refs"];
	        this.cycle = source["cycle"];
	        this.revert = source["revert"];
	        this.fixup = source["fixup"];
	        this.bot = source["bot"];
	    }
	}
	
	
	export class SignatureCheck {
	    id: string;
	    status: string;
	    signer: string;
	    key: string;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new SignatureCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.signer = source["signer"];
	        this.key = source["key"];
	        this.format = source["format"];
	    }
	}
	export class Signing {
	    enabled: boolean;
	    format: string;
	    key: string;
	
	    static createFrom(source: any = {}) {
	        return new Signing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.format = source["format"];
	        this.key = source["key"];
	    }
	}
	export class Status {
	    head: string;
	    oid: string;
	    upstream: string;
	    ahead: number;
	    behind: number;
	    files: FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.head = source["head"];
	        this.oid = source["oid"];
	        this.upstream = source["upstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.files = this.convertValues(source["files"], FileStatus);
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
	export class Submodule {
	    path: string;
	    commit: string;
	    state: string;
	    describe: string;
	
	    static createFrom(source: any = {}) {
	        return new Submodule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.commit = source["commit"];
	        this.state = source["state"];
	        this.describe = source["describe"];
	    }
	}
	export class TagInfo {
	    name: string;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new TagInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.time = source["time"];
	    }
	}
	
	export class TrailerHints {
	    keys: string[];
	    people: string[];
	
	    static createFrom(source: any = {}) {
	        return new TrailerHints(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = source["keys"];
	        this.people = source["people"];
	    }
	}
	export class Worktree {
	    path: string;
	    head: string;
	    branch: string;
	    main: boolean;
	    locked: boolean;
	    prunable: boolean;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Worktree(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.head = source["head"];
	        this.branch = source["branch"];
	        this.main = source["main"];
	        this.locked = source["locked"];
	        this.prunable = source["prunable"];
	        this.current = source["current"];
	    }
	}

}

export namespace gitconfig {
	
	export class Backup {
	    id: string;
	    // Go type: time
	    time: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Backup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = this.convertValues(source["time"], null);
	        this.size = source["size"];
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

export namespace graph {
	
	export class Seg {
	    from: number;
	    to: number;
	    color: number;
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new Seg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.color = source["color"];
	        this.kind = source["kind"];
	    }
	}
	export class Row {
	    index: number;
	    col: number;
	    color: number;
	    up: Seg[];
	    down: Seg[];
	    width: number;
	    kind?: string;
	    id: string;
	    parents: number;
	    author: string;
	    authorTime: number;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new Row(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.col = source["col"];
	        this.color = source["color"];
	        this.up = this.convertValues(source["up"], Seg);
	        this.down = this.convertValues(source["down"], Seg);
	        this.width = source["width"];
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.parents = source["parents"];
	        this.author = source["author"];
	        this.authorTime = source["authorTime"];
	        this.subject = source["subject"];
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
	export class SearchResult {
	    rows: number[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = source["rows"];
	        this.total = source["total"];
	    }
	}

}

export namespace links {
	
	export class Rule {
	    name: string;
	    pattern: string;
	    url: string;
	    in: string[];
	    shipped: boolean;
	    enabled: boolean;
	    scope: string;
	    shadowed?: boolean;
	    problem?: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pattern = source["pattern"];
	        this.url = source["url"];
	        this.in = source["in"];
	        this.shipped = source["shipped"];
	        this.enabled = source["enabled"];
	        this.scope = source["scope"];
	        this.shadowed = source["shadowed"];
	        this.problem = source["problem"];
	    }
	}
	export class Config {
	    rules: Rule[];
	    web: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], Rule);
	        this.web = source["web"];
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
	export class Match {
	    text: string;
	    href: string;
	    rule: string;
	
	    static createFrom(source: any = {}) {
	        return new Match(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.href = source["href"];
	        this.rule = source["rule"];
	    }
	}

}

export namespace session {
	
	export class Focus {
	    branch: string;
	    stack: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Focus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.stack = source["stack"];
	    }
	}

}

export namespace settings {
	
	export class Group {
	    name: string;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.paths = source["paths"];
	    }
	}

}

