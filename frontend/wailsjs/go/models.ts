export namespace app {
	
	export class AreaCommit {
	    sha: string;
	    subject: string;
	    when: number;
	
	    static createFrom(source: any = {}) {
	        return new AreaCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sha = source["sha"];
	        this.subject = source["subject"];
	        this.when = source["when"];
	    }
	}
	export class Arrived {
	    commits: string[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Arrived(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commits = source["commits"];
	        this.total = source["total"];
	    }
	}
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
	    linesUnknown?: boolean;
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
	        this.linesUnknown = source["linesUnknown"];
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
	    botAuthors: string[];
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
	        this.botAuthors = source["botAuthors"];
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
	export class AuthorArea {
	    path: string;
	    name: string;
	    files: boolean;
	    commits: number;
	    updated: number;
	    folders: number;
	    recent: AreaCommit[];
	
	    static createFrom(source: any = {}) {
	        return new AuthorArea(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.files = source["files"];
	        this.commits = source["commits"];
	        this.updated = source["updated"];
	        this.folders = source["folders"];
	        this.recent = this.convertValues(source["recent"], AreaCommit);
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
	export class AuthorAreas {
	    under: string;
	    areas: AuthorArea[];
	    more: number;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthorAreas(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.under = source["under"];
	        this.areas = this.convertValues(source["areas"], AuthorArea);
	        this.more = source["more"];
	        this.commits = source["commits"];
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
	export class AuthorCard {
	    shown: git.Identity;
	    written: git.Identity;
	    lines: git.MailmapRef[];
	    bot: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuthorCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shown = this.convertValues(source["shown"], git.Identity);
	        this.written = this.convertValues(source["written"], git.Identity);
	        this.lines = this.convertValues(source["lines"], git.MailmapRef);
	        this.bot = source["bot"];
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
	export class AuthorIdentity {
	    name: string;
	    email: string;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthorIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.commits = source["commits"];
	    }
	}
	export class MapAuthor {
	    key: string;
	    name: string;
	    email: string;
	    identities: AuthorIdentity[];
	    commits: number;
	    authored: number;
	    coAuthored: number;
	    updated: number;
	    bot: boolean;
	    you: boolean;
	    trunk: number;
	    branches: MapNode[];
	    moreBranches: number;
	
	    static createFrom(source: any = {}) {
	        return new MapAuthor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.identities = this.convertValues(source["identities"], AuthorIdentity);
	        this.commits = source["commits"];
	        this.authored = source["authored"];
	        this.coAuthored = source["coAuthored"];
	        this.updated = source["updated"];
	        this.bot = source["bot"];
	        this.you = source["you"];
	        this.trunk = source["trunk"];
	        this.branches = this.convertValues(source["branches"], MapNode);
	        this.moreBranches = source["moreBranches"];
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
	export class MapNode {
	    name: string;
	    kind: string;
	    commit: string;
	    commits: number;
	    updated: number;
	    parent: string;
	    stacked?: boolean;
	    worktree?: string;
	    head: boolean;
	    branchKind?: string;
	    tags?: number;
	    branches?: number;
	
	    static createFrom(source: any = {}) {
	        return new MapNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.commit = source["commit"];
	        this.commits = source["commits"];
	        this.updated = source["updated"];
	        this.parent = source["parent"];
	        this.stacked = source["stacked"];
	        this.worktree = source["worktree"];
	        this.head = source["head"];
	        this.branchKind = source["branchKind"];
	        this.tags = source["tags"];
	        this.branches = source["branches"];
	    }
	}
	export class AuthorMap {
	    trunk: MapNode;
	    authors: MapAuthor[];
	    people: number;
	    bots: number;
	    commits: number;
	    since: number;
	    uncountedBranches: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthorMap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trunk = this.convertValues(source["trunk"], MapNode);
	        this.authors = this.convertValues(source["authors"], MapAuthor);
	        this.people = source["people"];
	        this.bots = source["bots"];
	        this.commits = source["commits"];
	        this.since = source["since"];
	        this.uncountedBranches = source["uncountedBranches"];
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
	export class BlameNeeds {
	    partial: boolean;
	    versions: number;
	    missing: string[];
	
	    static createFrom(source: any = {}) {
	        return new BlameNeeds(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.partial = source["partial"];
	        this.versions = source["versions"];
	        this.missing = source["missing"];
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
	export class BranchMap {
	    trunk: MapNode;
	    nodes: MapNode[];
	    branches: number;
	    tags: number;
	    moreBranches: number;
	
	    static createFrom(source: any = {}) {
	        return new BranchMap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trunk = this.convertValues(source["trunk"], MapNode);
	        this.nodes = this.convertValues(source["nodes"], MapNode);
	        this.branches = source["branches"];
	        this.tags = source["tags"];
	        this.moreBranches = source["moreBranches"];
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
	
	export class BranchWebBase {
	    remote: string;
	    branch: string;
	    compare: string;
	
	    static createFrom(source: any = {}) {
	        return new BranchWebBase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.branch = source["branch"];
	        this.compare = source["compare"];
	    }
	}
	export class BranchWeb {
	    branch: string;
	    bases: BranchWebBase[];
	
	    static createFrom(source: any = {}) {
	        return new BranchWeb(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.bases = this.convertValues(source["bases"], BranchWebBase);
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
	
	export class BuiltWith {
	    go: string;
	    wails: string;
	
	    static createFrom(source: any = {}) {
	        return new BuiltWith(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.go = source["go"];
	        this.wails = source["wails"];
	    }
	}
	export class CareKey {
	    key: trailerindex.Key;
	    where: string;
	    kindSaid: boolean;
	    whereSaid: boolean;
	    also: string[];
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new CareKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = this.convertValues(source["key"], trailerindex.Key);
	        this.where = source["where"];
	        this.kindSaid = source["kindSaid"];
	        this.whereSaid = source["whereSaid"];
	        this.also = source["also"];
	        this.scope = source["scope"];
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
	    countsMissing?: string[];
	    author: git.Identity;
	    committer: git.Identity;
	    edge?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CommitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = this.convertValues(source["commit"], git.CommitDetail);
	        this.files = this.convertValues(source["files"], git.FileChange);
	        this.countsMissing = source["countsMissing"];
	        this.author = this.convertValues(source["author"], git.Identity);
	        this.committer = this.convertValues(source["committer"], git.Identity);
	        this.edge = source["edge"];
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
	export class ConflictContext {
	    mergeTool: string;
	    clean: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConflictContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mergeTool = source["mergeTool"];
	        this.clean = source["clean"];
	    }
	}
	export class ConflictPlace {
	    ours: string;
	    base: string;
	    theirs: string;
	    hasBase: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConflictPlace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ours = source["ours"];
	        this.base = source["base"];
	        this.theirs = source["theirs"];
	        this.hasBase = source["hasBase"];
	    }
	}
	export class ConflictReading {
	    language: string;
	    fallback: string;
	    suggestions: semdiff.Suggestion[];
	    driver: string;
	    driverCommand: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictReading(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.fallback = source["fallback"];
	        this.suggestions = this.convertValues(source["suggestions"], semdiff.Suggestion);
	        this.driver = source["driver"];
	        this.driverCommand = source["driverCommand"];
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
	export class MergeDriver {
	    name: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new MergeDriver(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	    }
	}
	export class DriverSettled {
	    path: string;
	    driver: string;
	
	    static createFrom(source: any = {}) {
	        return new DriverSettled(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.driver = source["driver"];
	    }
	}
	export class ConflictTally {
	    path: string;
	    places: number;
	    suggested: number;
	    language: string;
	    driver: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictTally(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.places = source["places"];
	        this.suggested = source["suggested"];
	        this.language = source["language"];
	        this.driver = source["driver"];
	    }
	}
	export class ConflictStop {
	    files: ConflictTally[];
	    settled: DriverSettled[];
	    drivers: MergeDriver[];
	
	    static createFrom(source: any = {}) {
	        return new ConflictStop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ConflictTally);
	        this.settled = this.convertValues(source["settled"], DriverSettled);
	        this.drivers = this.convertValues(source["drivers"], MergeDriver);
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
	
	
	export class DroppedCommit {
	    commit: string;
	    files: string[];
	    fileCount: number;
	    added: number;
	    deleted: number;
	
	    static createFrom(source: any = {}) {
	        return new DroppedCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.files = source["files"];
	        this.fileCount = source["fileCount"];
	        this.added = source["added"];
	        this.deleted = source["deleted"];
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
	export class FileDiff {
	    contents: git.FileContents;
	    diff?: semdiff.Result;
	
	    static createFrom(source: any = {}) {
	        return new FileDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contents = this.convertValues(source["contents"], git.FileContents);
	        this.diff = this.convertValues(source["diff"], semdiff.Result);
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
	export class FileHistoryResult {
	    commits: git.FileCommit[];
	    stop?: git.HistoryStop;
	
	    static createFrom(source: any = {}) {
	        return new FileHistoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commits = this.convertValues(source["commits"], git.FileCommit);
	        this.stop = this.convertValues(source["stop"], git.HistoryStop);
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
	export class FilterPick {
	    key: string;
	    value: string;
	    keyLabel: string;
	    valueLabel: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new FilterPick(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.keyLabel = source["keyLabel"];
	        this.valueLabel = source["valueLabel"];
	        this.kind = source["kind"];
	    }
	}
	export class FindCounted {
	    lines: number;
	    files: number;
	    superseded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FindCounted(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = source["lines"];
	        this.files = source["files"];
	        this.superseded = source["superseded"];
	    }
	}
	export class FindResult {
	    commit: string;
	    lines: number;
	    files: number;
	    truncated: boolean;
	    ms: number;
	    superseded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FindResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.lines = source["lines"];
	        this.files = source["files"];
	        this.truncated = source["truncated"];
	        this.ms = source["ms"];
	        this.superseded = source["superseded"];
	    }
	}
	export class ForecastCommit {
	    id: string;
	    subject: string;
	    author: string;
	    fixupOf: string;
	
	    static createFrom(source: any = {}) {
	        return new ForecastCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.subject = source["subject"];
	        this.author = source["author"];
	        this.fixupOf = source["fixupOf"];
	    }
	}
	export class ForecastConflict {
	    path: string;
	    places: number;
	    combinable: number;
	
	    static createFrom(source: any = {}) {
	        return new ForecastConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.places = source["places"];
	        this.combinable = source["combinable"];
	    }
	}
	export class GraphPage {
	    rows: graph.Row[];
	    refs: Record<string, Array<git.Ref>>;
	    total: number;
	    loading: boolean;
	    paused: boolean;
	    estimate: number;
	    chips?: Record<string, Array<trailerindex.Chip>>;
	
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
	        this.chips = this.convertValues(source["chips"], Array<trailerindex.Chip>, true);
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
	export class GraphViews {
	    view: string;
	    experimental: string[];
	
	    static createFrom(source: any = {}) {
	        return new GraphViews(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = source["view"];
	        this.experimental = source["experimental"];
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
	export class HistoryState {
	    options: git.HistoryOptions;
	    found: number;
	    searched: number;
	    total: number;
	    backTo: number;
	    running: boolean;
	    stopped: boolean;
	    ms: number;
	    estimateMs: number;
	    problem: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.options = this.convertValues(source["options"], git.HistoryOptions);
	        this.found = source["found"];
	        this.searched = source["searched"];
	        this.total = source["total"];
	        this.backTo = source["backTo"];
	        this.running = source["running"];
	        this.stopped = source["stopped"];
	        this.ms = source["ms"];
	        this.estimateMs = source["estimateMs"];
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
	    linesMissing: number;
	
	    static createFrom(source: any = {}) {
	        return new HotspotsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window = this.convertValues(source["window"], InsightsWindow);
	        this.files = this.convertValues(source["files"], HotFile);
	        this.rework = this.convertValues(source["rework"], git.ShippedItem);
	        this.linesMissing = source["linesMissing"];
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
	export class LaneName {
	    color: number;
	    row: number;
	    commit: string;
	    refs: git.Ref[];
	    merge?: string;
	
	    static createFrom(source: any = {}) {
	        return new LaneName(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.color = source["color"];
	        this.row = source["row"];
	        this.commit = source["commit"];
	        this.refs = this.convertValues(source["refs"], git.Ref);
	        this.merge = source["merge"];
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
	export class MailmapText {
	    text: string;
	    edited: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MailmapText(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.edited = source["edited"];
	    }
	}
	
	export class MapGroup {
	    nodes: MapNode[];
	    more: number;
	
	    static createFrom(source: any = {}) {
	        return new MapGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], MapNode);
	        this.more = source["more"];
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
	
	
	export class MergePreview {
	    missing?: string[];
	    into: string;
	    alreadyMerged: boolean;
	    fastForward: boolean;
	    commits: ForecastCommit[];
	    commitCount: number;
	    intoOnly: number;
	    files: number;
	    conflicts: ForecastConflict[];
	    unknown: boolean;
	    pastEdge?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MergePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.missing = source["missing"];
	        this.into = source["into"];
	        this.alreadyMerged = source["alreadyMerged"];
	        this.fastForward = source["fastForward"];
	        this.commits = this.convertValues(source["commits"], ForecastCommit);
	        this.commitCount = source["commitCount"];
	        this.intoOnly = source["intoOnly"];
	        this.files = source["files"];
	        this.conflicts = this.convertValues(source["conflicts"], ForecastConflict);
	        this.unknown = source["unknown"];
	        this.pastEdge = source["pastEdge"];
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
	export class PersonIdentity {
	    name: string;
	    email: string;
	    commits: number;
	    coAuthored: number;
	    lines: git.MailmapRef[];
	
	    static createFrom(source: any = {}) {
	        return new PersonIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.commits = source["commits"];
	        this.coAuthored = source["coAuthored"];
	        this.lines = this.convertValues(source["lines"], git.MailmapRef);
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
	export class Person {
	    name: string;
	    email: string;
	    commits: number;
	    coAuthored: number;
	    bot: boolean;
	    identities: PersonIdentity[];
	
	    static createFrom(source: any = {}) {
	        return new Person(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.commits = source["commits"];
	        this.coAuthored = source["coAuthored"];
	        this.bot = source["bot"];
	        this.identities = this.convertValues(source["identities"], PersonIdentity);
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
	export class PeopleList {
	    people: Person[];
	    matching: number;
	    everyone: number;
	    bots: number;
	    identities: number;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new PeopleList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.people = this.convertValues(source["people"], Person);
	        this.matching = source["matching"];
	        this.everyone = source["everyone"];
	        this.bots = source["bots"];
	        this.identities = source["identities"];
	        this.commits = source["commits"];
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
	export class PeopleRequest {
	    mailmap: MailmapText;
	    query: string;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new PeopleRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mailmap = this.convertValues(source["mailmap"], MailmapText);
	        this.query = source["query"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
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
	    experimentalViews: string[];
	    updateChecks: boolean;
	    gitPath: string;
	    gitInUse: string;
	    cloneDir: string;
	    zoom: number;
	    forgeColors: boolean;
	    diffAlgorithm: string;
	    diffRibbon: boolean;
	    diffMoves: boolean;
	    diffCode: boolean;
	    diffHideFormatting: boolean;
	    languages: string[];
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.graphDensity = source["graphDensity"];
	        this.experimentalViews = source["experimentalViews"];
	        this.updateChecks = source["updateChecks"];
	        this.gitPath = source["gitPath"];
	        this.gitInUse = source["gitInUse"];
	        this.cloneDir = source["cloneDir"];
	        this.zoom = source["zoom"];
	        this.forgeColors = source["forgeColors"];
	        this.diffAlgorithm = source["diffAlgorithm"];
	        this.diffRibbon = source["diffRibbon"];
	        this.diffMoves = source["diffMoves"];
	        this.diffCode = source["diffCode"];
	        this.diffHideFormatting = source["diffHideFormatting"];
	        this.languages = source["languages"];
	    }
	}
	export class PreviewImage {
	    url: string;
	    missing?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PreviewImage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.missing = source["missing"];
	    }
	}
	export class ProblemAction {
	    kind: string;
	    label: string;
	    url: string;
	    names: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProblemAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.url = source["url"];
	        this.names = source["names"];
	    }
	}
	export class Problem {
	    title: string;
	    detail: string;
	    quote: string;
	    actions: ProblemAction[];
	    output: string;
	    hidden: number;
	    full: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Problem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.detail = source["detail"];
	        this.quote = source["quote"];
	        this.actions = this.convertValues(source["actions"], ProblemAction);
	        this.output = source["output"];
	        this.hidden = source["hidden"];
	        this.full = source["full"];
	        this.path = source["path"];
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
	
	export class PushSetting {
	    key: string;
	    here: string;
	    inUse: string;
	
	    static createFrom(source: any = {}) {
	        return new PushSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.here = source["here"];
	        this.inUse = source["inUse"];
	    }
	}
	export class PushSettings {
	    scope: string;
	    settings: PushSetting[];
	    remotes: string[];
	    fetchAfterPush: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PushSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.settings = this.convertValues(source["settings"], PushSetting);
	        this.remotes = source["remotes"];
	        this.fetchAfterPush = source["fetchAfterPush"];
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
	export class RebasePlanPreview {
	    missing?: string[];
	    stopsAt: string;
	    conflicts: ForecastConflict[];
	    incomplete: boolean;
	    pushed: boolean;
	    upstream: string;
	    dropped: DroppedCommit[];
	
	    static createFrom(source: any = {}) {
	        return new RebasePlanPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.missing = source["missing"];
	        this.stopsAt = source["stopsAt"];
	        this.conflicts = this.convertValues(source["conflicts"], ForecastConflict);
	        this.incomplete = source["incomplete"];
	        this.pushed = source["pushed"];
	        this.upstream = source["upstream"];
	        this.dropped = this.convertValues(source["dropped"], DroppedCommit);
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
	export class RebasePreview {
	    missing?: string[];
	    branch: string;
	    commits: ForecastCommit[];
	    commitCount: number;
	    fixups: number;
	    upToDate: boolean;
	    stopsAt: string;
	    stopsAtSubject: string;
	    stopsAtNumber: number;
	    conflicts: ForecastConflict[];
	    incomplete: boolean;
	    pushed: boolean;
	    upstream: string;
	    pastEdge?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RebasePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.missing = source["missing"];
	        this.branch = source["branch"];
	        this.commits = this.convertValues(source["commits"], ForecastCommit);
	        this.commitCount = source["commitCount"];
	        this.fixups = source["fixups"];
	        this.upToDate = source["upToDate"];
	        this.stopsAt = source["stopsAt"];
	        this.stopsAtSubject = source["stopsAtSubject"];
	        this.stopsAtNumber = source["stopsAtNumber"];
	        this.conflicts = this.convertValues(source["conflicts"], ForecastConflict);
	        this.incomplete = source["incomplete"];
	        this.pushed = source["pushed"];
	        this.upstream = source["upstream"];
	        this.pastEdge = source["pastEdge"];
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
	export class RemoteBranchStatus {
	    base: string;
	    ahead: number;
	    behind: number;
	
	    static createFrom(source: any = {}) {
	        return new RemoteBranchStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base = source["base"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	    }
	}
	export class WebBase {
	    remote: string;
	    branch: string;
	
	    static createFrom(source: any = {}) {
	        return new WebBase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.branch = source["branch"];
	    }
	}
	export class RemoteWeb {
	    remote: string;
	    forge: string;
	    repo: string;
	    branches: string;
	    defaultBranch: string;
	    cloneHttps: string;
	    cloneSsh: string;
	    bases: WebBase[];
	
	    static createFrom(source: any = {}) {
	        return new RemoteWeb(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.forge = source["forge"];
	        this.repo = source["repo"];
	        this.branches = source["branches"];
	        this.defaultBranch = source["defaultBranch"];
	        this.cloneHttps = source["cloneHttps"];
	        this.cloneSsh = source["cloneSsh"];
	        this.bases = this.convertValues(source["bases"], WebBase);
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
	export class RepoPreferences {
	    kinds: KindPatterns[];
	    hide: string[];
	    hideKinds: string[];
	    showKinds: string[];
	    foldableKinds: string[];
	    botAuthors: string[];
	    builtInBotAuthor: string;
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
	        this.showKinds = source["showKinds"];
	        this.foldableKinds = source["foldableKinds"];
	        this.botAuthors = source["botAuthors"];
	        this.builtInBotAuthor = source["builtInBotAuthor"];
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
	    ours: string;
	    theirs: string;
	    commit: string;
	    subject: string;
	    origHead: string;
	    written: string[];
	
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
	        this.ours = source["ours"];
	        this.theirs = source["theirs"];
	        this.commit = source["commit"];
	        this.subject = source["subject"];
	        this.origHead = source["origHead"];
	        this.written = source["written"];
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
	export class RulerMarks {
	    loaded: number;
	    months: graph.MonthMark[];
	    tips: number[];
	    tags: number[];
	    notes: number[];
	
	    static createFrom(source: any = {}) {
	        return new RulerMarks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loaded = source["loaded"];
	        this.months = this.convertValues(source["months"], graph.MonthMark);
	        this.tips = source["tips"];
	        this.tags = source["tags"];
	        this.notes = source["notes"];
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
	export class SamePersonRequest {
	    identities: git.Identity[];
	    name: string;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new SamePersonRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.identities = this.convertValues(source["identities"], git.Identity);
	        this.name = source["name"];
	        this.email = source["email"];
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
	    peopleTab: string;
	    peopleTabId: string;
	
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
	        this.peopleTab = source["peopleTab"];
	        this.peopleTabId = source["peopleTabId"];
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
	    linesMissing: number;
	
	    static createFrom(source: any = {}) {
	        return new ShippedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window = this.convertValues(source["window"], InsightsWindow);
	        this.items = this.convertValues(source["items"], git.ShippedItem);
	        this.linesMissing = source["linesMissing"];
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
	export class SuggestedPerson {
	    name: string;
	    email: string;
	    commits: number;
	    coAuthored: number;
	    bot: boolean;
	    identities: git.Identity[];
	
	    static createFrom(source: any = {}) {
	        return new SuggestedPerson(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.commits = source["commits"];
	        this.coAuthored = source["coAuthored"];
	        this.bot = source["bot"];
	        this.identities = this.convertValues(source["identities"], git.Identity);
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
	export class Suggestion {
	    key: string;
	    kind: string;
	    people: SuggestedPerson[];
	    why?: string;
	
	    static createFrom(source: any = {}) {
	        return new Suggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.people = this.convertValues(source["people"], SuggestedPerson);
	        this.why = source["why"];
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
	export class Suggestions {
	    offers: Suggestion[];
	    offered: number;
	    shared: Suggestion[];
	    sharedCount: number;
	    apart: Suggestion[];
	    apartCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Suggestions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.offers = this.convertValues(source["offers"], Suggestion);
	        this.offered = source["offered"];
	        this.shared = this.convertValues(source["shared"], Suggestion);
	        this.sharedCount = source["sharedCount"];
	        this.apart = this.convertValues(source["apart"], Suggestion);
	        this.apartCount = source["apartCount"];
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
	
	export class TidyItem {
	    kind: string;
	    name: string;
	    local?: boolean;
	    stale?: boolean;
	    unpushed?: number;
	    onlyHere?: number;
	    lastAuthor?: string;
	    lastDate?: number;
	    ticked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TidyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.local = source["local"];
	        this.stale = source["stale"];
	        this.unpushed = source["unpushed"];
	        this.onlyHere = source["onlyHere"];
	        this.lastAuthor = source["lastAuthor"];
	        this.lastDate = source["lastDate"];
	        this.ticked = source["ticked"];
	    }
	}
	export class TidyReport {
	    remote: string;
	    base: string;
	    quietDays: number;
	    items: TidyItem[];
	
	    static createFrom(source: any = {}) {
	        return new TidyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.base = source["base"];
	        this.quietDays = source["quietDays"];
	        this.items = this.convertValues(source["items"], TidyItem);
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
	export class TidyResult {
	    name: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TidyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.error = source["error"];
	    }
	}
	export class TrailerCare {
	    questions: trailerindex.Question[];
	    open: number;
	    people: trailerindex.Question[];
	    peopleOpen: number;
	    joined: trailerindex.Joined;
	    keys: CareKey[];
	    status: trailerindex.Status;
	
	    static createFrom(source: any = {}) {
	        return new TrailerCare(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.questions = this.convertValues(source["questions"], trailerindex.Question);
	        this.open = source["open"];
	        this.people = this.convertValues(source["people"], trailerindex.Question);
	        this.peopleOpen = source["peopleOpen"];
	        this.joined = this.convertValues(source["joined"], trailerindex.Joined);
	        this.keys = this.convertValues(source["keys"], CareKey);
	        this.status = this.convertValues(source["status"], trailerindex.Status);
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
	export class TrailerFilterState {
	    picks: FilterPick[];
	    only: boolean;
	    kept: number;
	    total: number;
	    history?: HistoryState;
	
	    static createFrom(source: any = {}) {
	        return new TrailerFilterState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.picks = this.convertValues(source["picks"], FilterPick);
	        this.only = source["only"];
	        this.kept = source["kept"];
	        this.total = source["total"];
	        this.history = this.convertValues(source["history"], HistoryState);
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
	export class TrailerKeys {
	    keys: trailerindex.Key[];
	    status: trailerindex.Status;
	
	    static createFrom(source: any = {}) {
	        return new TrailerKeys(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = this.convertValues(source["keys"], trailerindex.Key);
	        this.status = this.convertValues(source["status"], trailerindex.Status);
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
	export class TrailerPanelKey {
	    key: trailerindex.Key;
	    top: trailerindex.Value[];
	    more: number;
	
	    static createFrom(source: any = {}) {
	        return new TrailerPanelKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = this.convertValues(source["key"], trailerindex.Key);
	        this.top = this.convertValues(source["top"], trailerindex.Value);
	        this.more = source["more"];
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
	export class TrailerPanel {
	    keys: TrailerPanelKey[];
	    moreKeys: number;
	    held: trailerindex.Key[];
	    matches: number;
	    status: trailerindex.Status;
	    care: number;
	
	    static createFrom(source: any = {}) {
	        return new TrailerPanel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = this.convertValues(source["keys"], TrailerPanelKey);
	        this.moreKeys = source["moreKeys"];
	        this.held = this.convertValues(source["held"], trailerindex.Key);
	        this.matches = source["matches"];
	        this.status = this.convertValues(source["status"], trailerindex.Status);
	        this.care = source["care"];
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
	
	export class TrailerPin {
	    key: string;
	    label: string;
	    kind: string;
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new TrailerPin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.scope = source["scope"];
	    }
	}
	export class TrailerProgress {
	    path: string;
	    reading: boolean;
	    read: number;
	    total: number;
	    done: boolean;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new TrailerProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reading = source["reading"];
	        this.read = source["read"];
	        this.total = source["total"];
	        this.done = source["done"];
	        this.commits = source["commits"];
	    }
	}
	export class TrailerSuggestion {
	    key: string;
	    keyLabel: string;
	    value: string;
	    label: string;
	    kind: string;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new TrailerSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.keyLabel = source["keyLabel"];
	        this.value = source["value"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.commits = source["commits"];
	    }
	}
	export class TrailerSuggestions {
	    items: TrailerSuggestion[];
	    match: string;
	    status: trailerindex.Status;
	
	    static createFrom(source: any = {}) {
	        return new TrailerSuggestions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], TrailerSuggestion);
	        this.match = source["match"];
	        this.status = this.convertValues(source["status"], trailerindex.Status);
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
	export class TrailerValues {
	    values: trailerindex.Value[];
	    matching: number;
	
	    static createFrom(source: any = {}) {
	        return new TrailerValues(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.values = this.convertValues(source["values"], trailerindex.Value);
	        this.matching = source["matching"];
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
	export class TrustQuestion {
	    path: string;
	    name: string;
	    findings: trust.Finding[];
	
	    static createFrom(source: any = {}) {
	        return new TrustQuestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.findings = this.convertValues(source["findings"], trust.Finding);
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
	
	export class WhoAnswer {
	    written: git.Identity;
	    shown: git.Identity;
	    lines: git.MailmapRef[];
	    commits: number;
	    coAuthored: number;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WhoAnswer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = this.convertValues(source["written"], git.Identity);
	        this.shown = this.convertValues(source["shown"], git.Identity);
	        this.lines = this.convertValues(source["lines"], git.MailmapRef);
	        this.commits = source["commits"];
	        this.coAuthored = source["coAuthored"];
	        this.known = source["known"];
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
	export class ApartPlan {
	    text: string;
	    removed: number[];
	    added: string[];
	    kept: Identity[];
	    lost: Identity[];
	
	    static createFrom(source: any = {}) {
	        return new ApartPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.removed = source["removed"];
	        this.added = source["added"];
	        this.kept = this.convertValues(source["kept"], Identity);
	        this.lost = this.convertValues(source["lost"], Identity);
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
	export class AuthorHistory {
	    authored: number;
	    coAuthored: number;
	    first: number;
	    last: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthorHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authored = source["authored"];
	        this.coAuthored = source["coAuthored"];
	        this.first = source["first"];
	        this.last = source["last"];
	    }
	}
	export class BackfillCost {
	    size: number;
	    missing: number;
	
	    static createFrom(source: any = {}) {
	        return new BackfillCost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.size = source["size"];
	        this.missing = source["missing"];
	    }
	}
	export class BlameLine {
	    commit: string;
	    author: string;
	    time: number;
	    summary: string;
	    line: string;
	    origLine: number;
	    edge?: boolean;
	
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
	        this.edge = source["edge"];
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
	export class CloneOptions {
	    depth: number;
	    since: number;
	    partial: boolean;
	    branch: string;
	    submodules: boolean;
	    folders: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CloneOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.depth = source["depth"];
	        this.since = source["since"];
	        this.partial = source["partial"];
	        this.branch = source["branch"];
	        this.submodules = source["submodules"];
	        this.folders = source["folders"];
	    }
	}
	export class OneBranchShape {
	    remote: string;
	    branch: string;
	
	    static createFrom(source: any = {}) {
	        return new OneBranchShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.branch = source["branch"];
	    }
	}
	export class SparseShape {
	    cone: boolean;
	    folders: string[];
	    files: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new SparseShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cone = source["cone"];
	        this.folders = source["folders"];
	        this.files = source["files"];
	        this.total = source["total"];
	    }
	}
	export class PartialShape {
	    remote: string;
	    filter: string;
	
	    static createFrom(source: any = {}) {
	        return new PartialShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.filter = source["filter"];
	    }
	}
	export class ShallowShape {
	    since: number;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new ShallowShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.since = source["since"];
	        this.commits = source["commits"];
	    }
	}
	export class CloneShape {
	    shallow?: ShallowShape;
	    partial?: PartialShape;
	    sparse?: SparseShape;
	    oneBranch?: OneBranchShape;
	
	    static createFrom(source: any = {}) {
	        return new CloneShape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shallow = this.convertValues(source["shallow"], ShallowShape);
	        this.partial = this.convertValues(source["partial"], PartialShape);
	        this.sparse = this.convertValues(source["sparse"], SparseShape);
	        this.oneBranch = this.convertValues(source["oneBranch"], OneBranchShape);
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
	    diff3: string;
	
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
	        this.diff3 = source["diff3"];
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
	export class FetchRules {
	    skipFetchAll: boolean;
	    prune: boolean;
	    tagOpt: string;
	    allBranches: boolean;
	    patterns: string[];
	    other: string[];
	
	    static createFrom(source: any = {}) {
	        return new FetchRules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skipFetchAll = source["skipFetchAll"];
	        this.prune = source["prune"];
	        this.tagOpt = source["tagOpt"];
	        this.allBranches = source["allBranches"];
	        this.patterns = source["patterns"];
	        this.other = source["other"];
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
	    missing?: string[];
	
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
	        this.missing = source["missing"];
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
	    caseTwin?: boolean;
	
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
	        this.caseTwin = source["caseTwin"];
	    }
	}
	export class FolderCount {
	    path: string;
	    files: number;
	    direct: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.files = source["files"];
	        this.direct = source["direct"];
	    }
	}
	export class GrepOptions {
	    text: string;
	    matchCase: boolean;
	    wholeWord: boolean;
	    regex: boolean;
	    untracked: boolean;
	    rev: string;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new GrepOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.matchCase = source["matchCase"];
	        this.wholeWord = source["wholeWord"];
	        this.regex = source["regex"];
	        this.untracked = source["untracked"];
	        this.rev = source["rev"];
	        this.paths = source["paths"];
	    }
	}
	export class HeadState {
	    branch: string;
	    commit: string;
	
	    static createFrom(source: any = {}) {
	        return new HeadState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.commit = source["commit"];
	    }
	}
	export class HistoryOptions {
	    text: string;
	    matchCase: boolean;
	    regex: boolean;
	    lines: boolean;
	    paths: string[];
	    since: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.matchCase = source["matchCase"];
	        this.regex = source["regex"];
	        this.lines = source["lines"];
	        this.paths = source["paths"];
	        this.since = source["since"];
	    }
	}
	export class HistoryStop {
	    commit: string;
	    missing: string[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryStop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.missing = source["missing"];
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
	export class JoinPlan {
	    lines: string[];
	    at: number[];
	    text: string;
	    named: Identity[];
	    blocked: Identity[];
	
	    static createFrom(source: any = {}) {
	        return new JoinPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = source["lines"];
	        this.at = source["at"];
	        this.text = source["text"];
	        this.named = this.convertValues(source["named"], Identity);
	        this.blocked = this.convertValues(source["blocked"], Identity);
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
	export class Location {
	    place: string;
	    forge: string;
	    host: string;
	    port: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Location(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.place = source["place"];
	        this.forge = source["forge"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.path = source["path"];
	    }
	}
	export class MailmapNote {
	    line: number;
	    kind: string;
	    newName?: string;
	    newEmail?: string;
	    oldName?: string;
	    oldEmail?: string;
	    why?: string;
	    identities: number;
	    commits: number;
	    coAuthored: number;
	    same: number;
	    replaced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MailmapNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.kind = source["kind"];
	        this.newName = source["newName"];
	        this.newEmail = source["newEmail"];
	        this.oldName = source["oldName"];
	        this.oldEmail = source["oldEmail"];
	        this.why = source["why"];
	        this.identities = source["identities"];
	        this.commits = source["commits"];
	        this.coAuthored = source["coAuthored"];
	        this.same = source["same"];
	        this.replaced = source["replaced"];
	    }
	}
	export class MailmapRef {
	    source: string;
	    line: number;
	
	    static createFrom(source: any = {}) {
	        return new MailmapRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.line = source["line"];
	    }
	}
	export class MailmapSource {
	    source: string;
	    path: string;
	    exists: boolean;
	    editable: boolean;
	    lines: number;
	
	    static createFrom(source: any = {}) {
	        return new MailmapSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.editable = source["editable"];
	        this.lines = source["lines"];
	    }
	}
	export class MoreHistory {
	    commits: number;
	    since: number;
	    all: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MoreHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commits = source["commits"];
	        this.since = source["since"];
	        this.all = source["all"];
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
	
	
	export class PlaceOrigin {
	    commit: string;
	    author: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaceOrigin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit = source["commit"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	    }
	}
	export class PushPlan {
	    branch: string;
	    remote: string;
	    ref: string;
	    setUpstream: boolean;
	    why: string;
	    appDefaults: string[];
	    ask: boolean;
	    remotes: string[];
	    refusal?: string;
	
	    static createFrom(source: any = {}) {
	        return new PushPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.remote = source["remote"];
	        this.ref = source["ref"];
	        this.setUpstream = source["setUpstream"];
	        this.why = source["why"];
	        this.appDefaults = source["appDefaults"];
	        this.ask = source["ask"];
	        this.remotes = source["remotes"];
	        this.refusal = source["refusal"];
	    }
	}
	export class PushTarget {
	    url: string;
	    remote: string;
	
	    static createFrom(source: any = {}) {
	        return new PushTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.remote = source["remote"];
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
	    pushRemote?: string;
	
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
	        this.pushRemote = source["pushRemote"];
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
	    pushUrls: string[];
	    location: Location;
	    pushes: PushTarget[];
	    prune?: boolean;
	    tagOpt: string;
	    skipFetchAll: boolean;
	    fetchRefspecs: string[];
	    fetchedAt: number;
	    hidden?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Remote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fetchUrl = source["fetchUrl"];
	        this.pushUrl = source["pushUrl"];
	        this.pushUrls = source["pushUrls"];
	        this.location = this.convertValues(source["location"], Location);
	        this.pushes = this.convertValues(source["pushes"], PushTarget);
	        this.prune = source["prune"];
	        this.tagOpt = source["tagOpt"];
	        this.skipFetchAll = source["skipFetchAll"];
	        this.fetchRefspecs = source["fetchRefspecs"];
	        this.fetchedAt = source["fetchedAt"];
	        this.hidden = source["hidden"];
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
	export class RemoteHeads {
	    branches: string[];
	    default: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteHeads(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branches = source["branches"];
	        this.default = source["default"];
	    }
	}
	export class RemoteSettings {
	    pushDefault: string;
	    fetchPrune?: boolean;
	    pushMode: string;
	    autoSetupRemote?: boolean;
	    followTags?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RemoteSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pushDefault = source["pushDefault"];
	        this.fetchPrune = source["fetchPrune"];
	        this.pushMode = source["pushMode"];
	        this.autoSetupRemote = source["autoSetupRemote"];
	        this.followTags = source["followTags"];
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
	
	export class MonthMark {
	    row: number;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new MonthMark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.row = source["row"];
	        this.time = source["time"];
	    }
	}
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
	    pass?: number[];
	    passKind?: string[];
	    width: number;
	    kind?: string;
	    match?: boolean;
	    edge?: boolean;
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
	        this.pass = source["pass"];
	        this.passKind = source["passKind"];
	        this.width = source["width"];
	        this.kind = source["kind"];
	        this.match = source["match"];
	        this.edge = source["edge"];
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
	
	export class ForgeRule {
	    host: string;
	    forge: string;
	    web?: string;
	    scope: string;
	    shadowed?: boolean;
	    problem?: string;
	
	    static createFrom(source: any = {}) {
	        return new ForgeRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.forge = source["forge"];
	        this.web = source["web"];
	        this.scope = source["scope"];
	        this.shadowed = source["shadowed"];
	        this.problem = source["problem"];
	    }
	}
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
	    forges: ForgeRule[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], Rule);
	        this.web = source["web"];
	        this.forges = this.convertValues(source["forges"], ForgeRule);
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

export namespace semdiff {
	
	export class Span {
	    line: number;
	    from: number;
	    to: number;
	    kind?: string;
	    rename?: number;
	
	    static createFrom(source: any = {}) {
	        return new Span(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.kind = source["kind"];
	        this.rename = source["rename"];
	    }
	}
	export class Block {
	    oldStart: number;
	    oldEnd: number;
	    newStart: number;
	    newEnd: number;
	    kind?: string;
	    note?: string;
	    old?: Span[];
	    new?: Span[];
	
	    static createFrom(source: any = {}) {
	        return new Block(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldEnd = source["oldEnd"];
	        this.newStart = source["newStart"];
	        this.newEnd = source["newEnd"];
	        this.kind = source["kind"];
	        this.note = source["note"];
	        this.old = this.convertValues(source["old"], Span);
	        this.new = this.convertValues(source["new"], Span);
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
	export class Move {
	    oldStart: number;
	    oldEnd: number;
	    newStart: number;
	    newEnd: number;
	    reindented: boolean;
	    changed?: boolean;
	    edits?: Block[];
	    into?: string;
	
	    static createFrom(source: any = {}) {
	        return new Move(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldEnd = source["oldEnd"];
	        this.newStart = source["newStart"];
	        this.newEnd = source["newEnd"];
	        this.reindented = source["reindented"];
	        this.changed = source["changed"];
	        this.edits = this.convertValues(source["edits"], Block);
	        this.into = source["into"];
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
	export class Origin {
	    from: number;
	    to: number;
	    side: string;
	
	    static createFrom(source: any = {}) {
	        return new Origin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.side = source["side"];
	    }
	}
	export class Rename {
	    from: string;
	    to: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Rename(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.count = source["count"];
	    }
	}
	export class Result {
	    blocks: Block[];
	    moves: Move[];
	    renames: Rename[];
	    algorithm: string;
	    engine: string;
	    language: string;
	    fallback: string;
	    unparsed: boolean;
	    added: number;
	    removed: number;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.blocks = this.convertValues(source["blocks"], Block);
	        this.moves = this.convertValues(source["moves"], Move);
	        this.renames = this.convertValues(source["renames"], Rename);
	        this.algorithm = source["algorithm"];
	        this.engine = source["engine"];
	        this.language = source["language"];
	        this.fallback = source["fallback"];
	        this.unparsed = source["unparsed"];
	        this.added = source["added"];
	        this.removed = source["removed"];
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
	
	export class Stretch {
	    from: number;
	    to: number;
	
	    static createFrom(source: any = {}) {
	        return new Stretch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class Suggestion {
	    ok: boolean;
	    text: string;
	    reason: string;
	    origins: Origin[];
	    ours: Stretch[];
	    theirs: Stretch[];
	
	    static createFrom(source: any = {}) {
	        return new Suggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.text = source["text"];
	        this.reason = source["reason"];
	        this.origins = this.convertValues(source["origins"], Origin);
	        this.ours = this.convertValues(source["ours"], Stretch);
	        this.theirs = this.convertValues(source["theirs"], Stretch);
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

export namespace trailerindex {
	
	export class Chip {
	    key: string;
	    keyLabel: string;
	    value: string;
	    label: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Chip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.keyLabel = source["keyLabel"];
	        this.value = source["value"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	    }
	}
	export class Spelling {
	    text: string;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new Spelling(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.commits = source["commits"];
	    }
	}
	export class Key {
	    id: string;
	    label: string;
	    spellings: Spelling[];
	    kind: string;
	    commits: number;
	    lines: number;
	    values: number;
	    held: string;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Key(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.spellings = this.convertValues(source["spellings"], Spelling);
	        this.kind = source["kind"];
	        this.commits = source["commits"];
	        this.lines = source["lines"];
	        this.values = source["values"];
	        this.held = source["held"];
	        this.known = source["known"];
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
	export class Joined {
	    spellings: Key[];
	    several: number;
	    bots: number;
	
	    static createFrom(source: any = {}) {
	        return new Joined(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spellings = this.convertValues(source["spellings"], Key);
	        this.several = source["several"];
	        this.bots = source["bots"];
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
	
	export class Pick {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Pick(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Question {
	    kind: string;
	    key: string;
	    keyLabel: string;
	    commits: number;
	    other: string;
	    otherLabel: string;
	    otherCommits: number;
	    shared: number;
	    examples: string[];
	
	    static createFrom(source: any = {}) {
	        return new Question(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.key = source["key"];
	        this.keyLabel = source["keyLabel"];
	        this.commits = source["commits"];
	        this.other = source["other"];
	        this.otherLabel = source["otherLabel"];
	        this.otherCommits = source["otherCommits"];
	        this.shared = source["shared"];
	        this.examples = source["examples"];
	    }
	}
	
	export class Status {
	    reading: boolean;
	    read: number;
	    total: number;
	    done: boolean;
	    commits: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reading = source["reading"];
	        this.read = source["read"];
	        this.total = source["total"];
	        this.done = source["done"];
	        this.commits = source["commits"];
	    }
	}
	export class Value {
	    id: string;
	    label: string;
	    kind: string;
	    commits: number;
	    bot?: boolean;
	    href?: string;
	
	    static createFrom(source: any = {}) {
	        return new Value(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.commits = source["commits"];
	        this.bot = source["bot"];
	        this.href = source["href"];
	    }
	}

}

export namespace trust {
	
	export class Finding {
	    kind: string;
	    name: string;
	    value: string;
	    why: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.why = source["why"];
	    }
	}

}

