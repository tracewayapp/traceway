<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { ShieldCheck } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';

	interface Props {
		open: boolean;
		methodId: number | null;
		phoneLabel: string;
		codeJustSent?: boolean;
		onVerified: () => void;
	}

	let { open = $bindable(), methodId, phoneLabel, codeJustSent = false, onVerified }: Props = $props();

	let code = $state('');
	let loading = $state(false);
	let resending = $state(false);
	let error = $state('');
	let resendCountdown = $state(0);
	let countdownTimer: ReturnType<typeof setInterval> | null = null;

	function startCountdown() {
		resendCountdown = 60;
		if (countdownTimer) clearInterval(countdownTimer);
		countdownTimer = setInterval(() => {
			resendCountdown -= 1;
			if (resendCountdown <= 0 && countdownTimer) {
				clearInterval(countdownTimer);
				countdownTimer = null;
			}
		}, 1000);
	}

	$effect(() => {
		if (open) {
			code = '';
			error = '';
			if (codeJustSent) {
				startCountdown();
			} else {
				resendCountdown = 0;
			}
			return () => {
				if (countdownTimer) {
					clearInterval(countdownTimer);
					countdownTimer = null;
				}
			};
		}
	});

	async function handleSubmit() {
		if (methodId === null) return;
		loading = true;
		error = '';
		try {
			await api.post(`/contact-methods/${methodId}/verify`, { code: code.trim() });
			toast.success('Successfully verified the Contact Method', { position: 'top-center' });
			open = false;
			onVerified();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to verify the code';
		} finally {
			loading = false;
		}
	}

	async function resendCode() {
		if (methodId === null || resendCountdown > 0 || resending) return;
		resending = true;
		error = '';
		try {
			await api.post(`/contact-methods/${methodId}/resend-code`, {});
			toast.success('Verification code sent', { position: 'top-center' });
			startCountdown();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to resend the code';
		} finally {
			resending = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(isOpen) => (open = isOpen)}>
	<AlertDialog.Content class="max-w-md">
		<AlertDialog.Header>
			<AlertDialog.Title>Verify Phone Number</AlertDialog.Title>
			<AlertDialog.Description>
				{codeJustSent
					? `Enter the 6-digit code we texted to ${phoneLabel}`
					: `Enter the 6-digit code sent to ${phoneLabel}, or request a new one`}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<ErrorAlert {error} />

			<div class="space-y-2">
				<Label for="verify-code">Verification Code</Label>
				<Input
					id="verify-code"
					bind:value={code}
					inputmode="numeric"
					maxlength={6}
					autocomplete="one-time-code"
					placeholder="123456"
					class="text-center font-mono text-lg tracking-[0.3em]"
				/>
			</div>

			<div class="text-sm text-muted-foreground">
				{#if resendCountdown > 0}
					Didn't get it? Resend in {resendCountdown}s
				{:else}
					<button
						type="button"
						class="cursor-pointer text-primary hover:underline disabled:cursor-default disabled:opacity-50"
						disabled={resending}
						onclick={resendCode}
					>
						{resending ? 'Resending...' : 'Resend code'}
					</button>
				{/if}
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button onclick={handleSubmit} disabled={loading}>
				<ShieldCheck class="mr-2 h-4 w-4" />
				{loading ? 'Verifying...' : 'Verify Phone Number'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
