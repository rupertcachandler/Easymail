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
	export class Contact {
	    id: string;
	    accountId: string;
	    name: string;
	    email: string;
	    avatarUrl?: string;
	    // Go type: time
	    lastEmailAt: any;
	    emailCount: number;
	    isFavorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.avatarUrl = source["avatarUrl"];
	        this.lastEmailAt = this.convertValues(source["lastEmailAt"], null);
	        this.emailCount = source["emailCount"];
	        this.isFavorite = source["isFavorite"];
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

}

