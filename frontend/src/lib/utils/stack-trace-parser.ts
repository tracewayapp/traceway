export type StackFrame = {
	functionName: string | null;
	location: string;
	isLibrary: boolean;
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

function extractPackageName(location: string): string {
	const match = location.match(/node_modules\/([^/]+)/);
	return match ? match[1] : 'library';
}

export function parseStackTrace(raw: string): ParsedStackTrace {
	const lines = raw.split('\n');
	const frames: StackFrame[] = [];
	let firstFrameIndex = -1;

	const locationPattern = /^\s{4}.+:\d+:\d+$/;

	for (let i = 0; i < lines.length; i++) {
		if (locationPattern.test(lines[i])) {
			if (firstFrameIndex === -1) firstFrameIndex = i;

			const location = lines[i].trim();
			let functionName: string | null = null;

			if (i > 0) {
				const prevLine = lines[i - 1].trim();
				if (prevLine.endsWith('()')) {
					functionName = prevLine;
				}
			}

			frames.push({
				functionName,
				location,
				isLibrary: location.includes('node_modules')
			});
		}
	}

	const errorMessage = firstFrameIndex === -1
		? raw.trim()
		: lines.slice(0, firstFrameIndex - (frames[0]?.functionName ? 1 : 0)).join('\n').trim();

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
				packageName: extractPackageName(libraryFrames[0].location)
			});
		}
	}

	return { errorMessage, groups };
}
