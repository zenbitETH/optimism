// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

/**
 * @title ITournamentFactory
 * @notice Interface for the TournamentFactory contract
 */
interface ITournamentFactory {
    /**
     * @notice Creates a new tournament for dispute resolution
     * @param rootClaim The root claim being disputed
     * @param l2BlockNumber The L2 block number associated with this dispute
     * @param extraData Extra data for tournament creation (challenge period, bond size)
     * @return The address of the newly created tournament
     */
    function createTournament(
        bytes32 rootClaim,
        uint256 l2BlockNumber,
        bytes calldata extraData
    ) external returns (address);

    /**
     * @notice Checks if an address is a valid tournament created by this factory
     * @param tournamentAddr The address to check
     * @return True if the address is a valid tournament, false otherwise
     */
    function isTournament(address tournamentAddr) external view returns (bool);
}