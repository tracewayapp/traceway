export type StackFrame = {
	functionName: string | null;
	location: string;
	isLibrary: boolean;
	packageHint?: string;
};

export type FrameGroup = {
	type: 'app';
	frame: StackFrame;
} | {
	type: 'library';
	frames: StackFrame[];
	packageName: string;
};

export type ParsedStackTrace = {
	errorMessage: string;
	groups: FrameGroup[];
};

function isLibraryLocation(location: string): boolean {
	return (
		location.includes('node_modules') ||
		/^node:/.test(location) ||
		/^dart:/.test(location) ||
		/^third_party\//.test(location) ||
		/^lib\/ui\//.test(location) ||
		/^package:(flutter(\/|_)|collection\/)/.test(location) ||
		/SnapshotInstructions\+0x/.test(location)
	);
}

const javaFramePattern = /^\s*at\s+([\w$.<>]+)\(([^)]*)\)\s*$/;

const JAVA_SYSTEM_RE =
	/^(java\.|javax\.|jakarta\.|jdk\.|sun\.|com\.sun\.|kotlin\.|kotlinx\.|android\.|androidx\.|com\.android\.|com\.google\.android\.|dalvik\.|libcore\.|org\.json\.)/;

export function looksLikeJava(raw: string): boolean {
	let count = 0;
	for (const line of raw.split('\n')) {
		const m = line.match(javaFramePattern);
		if (m && !m[2].includes('/')) {
			if (++count >= 2) return true;
		}
	}
	return false;
}

function isJavaSystemClass(fqn: string): boolean {
	return JAVA_SYSTEM_RE.test(fqn);
}

function javaPackageLabel(fqn: string): string {
	const segs = fqn.split('.');
	if ((segs[0] === 'com' || segs[0] === 'org' || segs[0] === 'io') && segs.length > 1) {
		return `${segs[0]}.${segs[1]}`;
	}
	return segs[0] || 'system';
}

const goLocationPattern = /^[ \t]+(.+\.go):(\d+)(?:\s+\+0x[0-9a-fA-F]+)?\s*$/;
const goroutineHeaderPattern = /^\s*goroutine\s+\d+\s+\[[^\]]*\]:\s*$/;

export function looksLikeGo(raw: string): boolean {
	let locations = 0;
	let goroutine = false;
	for (const line of raw.split('\n')) {
		if (goLocationPattern.test(line)) {
			if (++locations >= 2) return true;
		} else if (goroutineHeaderPattern.test(line)) {
			goroutine = true;
		}
	}
	return locations >= 1 && goroutine;
}

function goFunctionName(rawFn: string): string {
	return rawFn
		.replace(/^created by\s+/, '')
		.replace(/\s+in goroutine\s+\d+\s*$/, '')
		.replace(/\([^()]*\)\s*$/, '')
		.trim();
}

function goFunctionPackage(fn: string): string {
	const lastSlash = fn.lastIndexOf('/');
	const dot = fn.indexOf('.', lastSlash + 1);
	if (dot < 0) return '';
	const pkg = fn.slice(0, dot);
	return pkg.startsWith('(') ? '' : pkg;
}

function isGoStdlibPackage(pkg: string): boolean {
	if (pkg === '' || pkg === 'main') return false;
	return !pkg.split('/')[0].includes('.');
}

function isGoDependencyPath(file: string): boolean {
	return file.includes('/pkg/mod/') || file.includes('/vendor/') || /@v\d/.test(file);
}

function isGoStdlibPath(file: string): boolean {
	if (/(?:^|\/)go\/src\//.test(file)) return true;
	if (file.startsWith('/')) return false;
	return !file.split('/')[0].includes('.');
}

function goDependencyHint(pkg: string, file: string): string {
	if (pkg.includes('.')) return pkg;
	const rest = file.match(/\/(?:pkg\/mod|vendor)\/(.+)/);
	if (rest) {
		const out: string[] = [];
		for (const seg of rest[1].split('/')) {
			const at = seg.indexOf('@');
			if (at >= 0) {
				out.push(seg.slice(0, at));
				break;
			}
			out.push(seg);
			if (out.length >= 3) break;
		}
		return out.join('/');
	}
	return pkg || 'dependency';
}

function goStdlibHint(pkg: string, file: string): string {
	if (pkg) return pkg;
	const rel = file.match(/\/go\/src\/(.+)/)?.[1] ?? file;
	const slash = rel.lastIndexOf('/');
	return slash >= 0 ? rel.slice(0, slash) : 'system';
}

function classifyGoFrame(fn: string, file: string): { isLibrary: boolean; hint?: string } {
	const pkg = goFunctionPackage(fn);
	if (isGoDependencyPath(file)) {
		return { isLibrary: true, hint: goDependencyHint(pkg, file) };
	}
	if (pkg !== 'main' && isGoStdlibPackage(pkg)) {
		return { isLibrary: true, hint: pkg };
	}
	if (isGoStdlibPath(file)) {
		return { isLibrary: true, hint: goStdlibHint(pkg, file) };
	}
	return { isLibrary: false };
}

function goShortFunction(fn: string): string {
	const slash = fn.lastIndexOf('/');
	return slash >= 0 ? fn.slice(slash + 1) : fn;
}

const IOS_SYSTEM_IMAGES = new Set([
	'CoreFoundation', 'Foundation', 'CFNetwork', 'Security', 'UIKitCore', 'UIKit', 'SwiftUI',
	'GraphicsServices', 'QuartzCore', 'CoreGraphics', 'CoreText', 'CoreData', 'CoreImage',
	'CoreAudio', 'CoreVideo', 'CoreServices', 'CoreMotion', 'CoreLocation', 'Metal', 'MetalKit',
	'ImageIO', 'Combine', 'Network', 'AudioToolbox', 'AVFoundation', 'WebKit', 'JavaScriptCore', 'dyld'
]);

function isIOSSystemImage(image: string): boolean {
	const img = image.trim();
	if (img === '' || img === '<unknown>') return true;
	if (/^lib.*\.dylib$/.test(img)) return true;
	if (/^libswift/.test(img)) return true;
	return IOS_SYSTEM_IMAGES.has(img);
}

function isIOSSystemLocation(location: string): boolean {
	const loc = location.trim();
	return loc === '' || loc === '<compiler-generated>' || loc === '<unknown>';
}

function isIOSEntryFunction(fn: string): boolean {
	const f = fn.trim();
	return f === 'main';
}

function extractPackageName(location: string): string {
	const iosImageMatch = location.match(/^(.+?)\+0x[0-9a-fA-F]+$/);
	if (iosImageMatch) return iosImageMatch[1];
	if (location === '<compiler-generated>' || location === '<unknown>') return 'system';

	const nodeModulesMatch = location.match(/node_modules\/([^/]+)/);
	if (nodeModulesMatch) return nodeModulesMatch[1];

	const nodeInternalMatch = location.match(/^node:[a-z_]+/);
	if (nodeInternalMatch) return nodeInternalMatch[0];

	const dartMatch = location.match(/^(package:[^/]+|dart:[^/]+)/);
	if (dartMatch) return dartMatch[1];

	if (/^third_party\//.test(location)) return 'dart sdk';
	if (/^lib\/ui\//.test(location)) return 'dart:ui';
	if (/SnapshotInstructions\+0x/.test(location)) return 'unresolved';

	return 'library';
}

export function parseStackTrace(
	raw: string,
	opts: { ios?: boolean; java?: boolean; go?: boolean } = {}
): ParsedStackTrace {
	const lines = raw.split('\n');
	const frames: StackFrame[] = [];
	let firstFrameIndex = -1;
	let messageEndIndex = -1;

	const locationPattern = /^\s*.+:\d+:\d+$/;
	const dartFramePattern = /^\s*#\d+\s+(.+?)\s+\((.+)\)\s*$/;
	const dartUnresolvedPattern = /^\s*#\d+\s+(\S+SnapshotInstructions\+0x[0-9a-fA-F]+)\s*$/;
	const iosResolvedPattern = /^\s*#\d+\s+(.+?)\s+\((.+)\)\s*$/;
	const iosUnresolvedPattern = /^\s*#\d+\s+(.+?)\+0x[0-9a-fA-F]+\s*$/;

	for (let i = 0; i < lines.length; i++) {
		if (opts.ios) {
			const iosResolved = lines[i].match(iosResolvedPattern);
			if (iosResolved) {
				const fn = iosResolved[1].trim();
				const location = iosResolved[2].trim();
				if (firstFrameIndex === -1) {
					firstFrameIndex = i;
					messageEndIndex = i;
				}
				frames.push({
					functionName: fn,
					location,
					isLibrary: isIOSSystemLocation(location) || isIOSEntryFunction(fn)
				});
				continue;
			}
			const iosUnresolved = lines[i].match(iosUnresolvedPattern);
			if (iosUnresolved) {
				if (firstFrameIndex === -1) {
					firstFrameIndex = i;
					messageEndIndex = i;
				}
				frames.push({
					functionName: null,
					location: lines[i].trim().replace(/^#\d+\s+/, ''),
					isLibrary: isIOSSystemImage(iosUnresolved[1].trim())
				});
				continue;
			}
		}

		if (opts.java) {
			const jm = lines[i].match(javaFramePattern);
			if (jm && !jm[2].includes('/')) {
				const fqMethod = jm[1];
				const source = jm[2].trim();
				const lastDot = fqMethod.lastIndexOf('.');
				const method = lastDot >= 0 ? fqMethod.slice(lastDot + 1) : fqMethod;
				const classFqn = lastDot >= 0 ? fqMethod.slice(0, lastDot) : '';
				const classSimple = classFqn.includes('.')
					? classFqn.slice(classFqn.lastIndexOf('.') + 1)
					: classFqn;
				const lib = isJavaSystemClass(fqMethod);
				if (firstFrameIndex === -1) {
					firstFrameIndex = i;
					messageEndIndex = i;
				}
				frames.push({
					functionName: classSimple ? `${classSimple}.${method}` : method,
					location: source,
					isLibrary: lib,
					packageHint: lib ? javaPackageLabel(fqMethod) : undefined
				});
				continue;
			}
		}

		if (opts.go) {
			const gm = lines[i].match(goLocationPattern);
			if (gm) {
				const location = `${gm[1]}:${gm[2]}`;
				let funcName: string | null = null;
				let funcIndex = -1;
				for (let j = i - 1; j >= 0; j--) {
					const prev = lines[j].trim();
					if (prev === '') continue;
					if (goroutineHeaderPattern.test(lines[j]) || goLocationPattern.test(lines[j])) break;
					funcName = prev;
					funcIndex = j;
					break;
				}
				const fullName = funcName ? goFunctionName(funcName) : '';
				const cls = classifyGoFrame(fullName, gm[1]);
				if (firstFrameIndex === -1) {
					firstFrameIndex = i;
					messageEndIndex = funcIndex !== -1 ? funcIndex : i;
				}
				frames.push({
					functionName: fullName ? goShortFunction(fullName) : null,
					location,
					isLibrary: cls.isLibrary,
					packageHint: cls.isLibrary ? cls.hint : undefined
				});
				continue;
			}
		}

		const dartMatch = lines[i].match(dartFramePattern);
		if (dartMatch) {
			const location = dartMatch[2].trim();
			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = i;
			}
			frames.push({
				functionName: dartMatch[1].trim(),
				location,
				isLibrary: isLibraryLocation(location)
			});
			continue;
		}

		const unresolvedMatch = lines[i].match(dartUnresolvedPattern);
		if (unresolvedMatch) {
			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = i;
			}
			frames.push({ functionName: null, location: unresolvedMatch[1].trim(), isLibrary: true });
			continue;
		}

		if (locationPattern.test(lines[i])) {
			const location = lines[i].trim();
			let functionName: string | null = null;
			let funcNameIndex = -1;

			for (let j = i - 1; j >= 0; j--) {
				const prevLine = lines[j].trim();
				if (prevLine === '') continue;
				if (!locationPattern.test(lines[j])) {
					const messageLike =
						firstFrameIndex === -1 && !prevLine.endsWith('()') && prevLine.includes(': ');
					if (!messageLike) {
						functionName = prevLine;
						funcNameIndex = j;
					}
				}
				break;
			}

			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = funcNameIndex !== -1 ? funcNameIndex : i;
			}

			frames.push({ functionName, location, isLibrary: isLibraryLocation(location) });
		}
	}

	const errorMessage = firstFrameIndex === -1
		? raw.trim()
		: lines
				.slice(0, messageEndIndex)
				.filter((l) => !goroutineHeaderPattern.test(l))
				.join('\n')
				.trim();

	const groups: FrameGroup[] = [];

	for (let i = 0; i < frames.length; i++) {
		const frame = frames[i];

		if (!frame.isLibrary) {
			groups.push({ type: 'app', frame });
		} else {
			const libraryFrames: StackFrame[] = [frame];
			while (i + 1 < frames.length && frames[i + 1].isLibrary) {
				i++;
				libraryFrames.push(frames[i]);
			}
			groups.push({
				type: 'library',
				frames: libraryFrames,
				packageName: libraryFrames[0].packageHint ?? extractPackageName(libraryFrames[0].location)
			});
		}
	}

	return { errorMessage, groups };
}
