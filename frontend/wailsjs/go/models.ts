export namespace models {
	
	export class Account {
	    id: string;
	    name: string;
	    email: string;
	    password: string;
	    serverUrl: string;
	    deviceId: string;
	    deviceType: string;
	    connected: boolean;
	    oofState?: string;
	    oofExternal?: string;
	    oofInternal?: string;
	    // Go type: time
	    lastSync: any;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.serverUrl = source["serverUrl"];
	        this.deviceId = source["deviceId"];
	        this.deviceType = source["deviceType"];
	        this.connected = source["connected"];
	        this.oofState = source["oofState"];
	        this.oofExternal = source["oofExternal"];
	        this.oofInternal = source["oofInternal"];
	        this.lastSync = this.convertValues(source["lastSync"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class CalendarEvent {
	    id: string;
	    accountId: string;
	    serverId: string;
	    subject: string;
	    location?: string;
	    // Go type: time
	    startTime: any;
	    // Go type: time
	    endTime: any;
	    allDayEvent: boolean;
	    organizerName?: string;
	    organizerEmail?: string;
	    attendees?: string[];
	    busyStatus: number;
	    sensitivity: number;
	    reminder?: number;
	    body?: string;
	    bodyType?: string;
	    recurrenceType?: number;
	    recurrenceUntil?: string;
	
	    static createFrom(source: any = {}) {
	        return new CalendarEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.serverId = source["serverId"];
	        this.subject = source["subject"];
	        this.location = source["location"];
	        this.startTime = this.convertValues(source["startTime"], null);
	        this.endTime = this.convertValues(source["endTime"], null);
	        this.allDayEvent = source["allDayEvent"];
	        this.organizerName = source["organizerName"];
	        this.organizerEmail = source["organizerEmail"];
	        this.attendees = source["attendees"];
	        this.busyStatus = source["busyStatus"];
	        this.sensitivity = source["sensitivity"];
	        this.reminder = source["reminder"];
	        this.body = source["body"];
	        this.bodyType = source["bodyType"];
	        this.recurrenceType = source["recurrenceType"];
	        this.recurrenceUntil = source["recurrenceUntil"];
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
	export class Contact {
	    id: string;
	    accountId: string;
	    name: string;
	    firstName?: string;
	    lastName?: string;
	    email: string;
	    email2?: string;
	    email3?: string;
	    phone?: string;
	    mobile?: string;
	    company?: string;
	    jobTitle?: string;
	    avatarUrl?: string;
	    // Go type: time
	    lastEmailAt: any;
	    emailCount: number;
	    isFavorite: boolean;
	    serverId?: string;
	
	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.firstName = source["firstName"];
	        this.lastName = source["lastName"];
	        this.email = source["email"];
	        this.email2 = source["email2"];
	        this.email3 = source["email3"];
	        this.phone = source["phone"];
	        this.mobile = source["mobile"];
	        this.company = source["company"];
	        this.jobTitle = source["jobTitle"];
	        this.avatarUrl = source["avatarUrl"];
	        this.lastEmailAt = this.convertValues(source["lastEmailAt"], null);
	        this.emailCount = source["emailCount"];
	        this.isFavorite = source["isFavorite"];
	        this.serverId = source["serverId"];
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
	export class EmailAttachment {
	    displayName?: string;
	    fileReference?: string;
	    contentId?: string;
	    isInline?: boolean;
	    method?: number;
	    estimatedDataSize?: number;
	
	    static createFrom(source: any = {}) {
	        return new EmailAttachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayName = source["displayName"];
	        this.fileReference = source["fileReference"];
	        this.contentId = source["contentId"];
	        this.isInline = source["isInline"];
	        this.method = source["method"];
	        this.estimatedDataSize = source["estimatedDataSize"];
	    }
	}
	export class Email {
	    id: string;
	    accountId: string;
	    folderId: string;
	    serverId: string;
	    from: string;
	    fromEmail: string;
	    to: string;
	    toEmails: string[];
	    cc?: string;
	    ccEmails?: string[];
	    subject: string;
	    threadTopic?: string;
	    // Go type: time
	    dateReceived: any;
	    isRead: boolean;
	    isFlagged: boolean;
	    importance: number;
	    hasAttachment: boolean;
	    attachments?: EmailAttachment[];
	    preview: string;
	    body?: string;
	    bodyType?: string;
	
	    static createFrom(source: any = {}) {
	        return new Email(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.folderId = source["folderId"];
	        this.serverId = source["serverId"];
	        this.from = source["from"];
	        this.fromEmail = source["fromEmail"];
	        this.to = source["to"];
	        this.toEmails = source["toEmails"];
	        this.cc = source["cc"];
	        this.ccEmails = source["ccEmails"];
	        this.subject = source["subject"];
	        this.threadTopic = source["threadTopic"];
	        this.dateReceived = this.convertValues(source["dateReceived"], null);
	        this.isRead = source["isRead"];
	        this.isFlagged = source["isFlagged"];
	        this.importance = source["importance"];
	        this.hasAttachment = source["hasAttachment"];
	        this.attachments = this.convertValues(source["attachments"], EmailAttachment);
	        this.preview = source["preview"];
	        this.body = source["body"];
	        this.bodyType = source["bodyType"];
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
	
	export class Folder {
	    id: string;
	    accountId: string;
	    serverId: string;
	    parentId?: string;
	    name: string;
	    type: number;
	    isHidden: boolean;
	    unreadCount: number;
	    // Go type: time
	    lastSync: any;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.serverId = source["serverId"];
	        this.parentId = source["parentId"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.isHidden = source["isHidden"];
	        this.unreadCount = source["unreadCount"];
	        this.lastSync = this.convertValues(source["lastSync"], null);
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
	export class OOFSettings {
	    state: string;
	    startTime?: string;
	    endTime?: string;
	    internal: string;
	    external: string;
	
	    static createFrom(source: any = {}) {
	        return new OOFSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.internal = source["internal"];
	        this.external = source["external"];
	    }
	}

}

export namespace struct { ServerID string "json:\"serverId\""; FolderID string "json:\"folderId\"" } {
	
	export class  {
	    serverId: string;
	    folderId: string;
	
	    static createFrom(source: any = {}) {
	        return new (source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverId = source["serverId"];
	        this.folderId = source["folderId"];
	    }
	}

}

export namespace struct { ServerID string "json:\"serverId\""; SrcFolderID string "json:\"srcFolderId\""; DstFolderID string "json:\"dstFolderId\"" } {
	
	export class  {
	    serverId: string;
	    srcFolderId: string;
	    dstFolderId: string;
	
	    static createFrom(source: any = {}) {
	        return new (source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverId = source["serverId"];
	        this.srcFolderId = source["srcFolderId"];
	        this.dstFolderId = source["dstFolderId"];
	    }
	}

}

